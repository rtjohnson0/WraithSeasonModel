"""Same-origin Whop app gateway. Premium data is never a static asset."""
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
import hashlib
import hmac
import json
import os
import re
import tempfile
import threading
import time

from fastapi import FastAPI, Request
from fastapi.responses import FileResponse, JSONResponse, Response
import jwt
from whop_sdk import Whop
from whop_sdk.lib.verify_user_token import verify_user_token

PRODUCT_ID = 'prod_bgvx5EzER8i1A'
STORE_URL = 'https://whop.com/wraithsznbets'
MAX_BYTES = 25 * 1024 * 1024


@dataclass(frozen=True)
class Settings:
    app_id: str
    resource_id: str
    api_key: str
    publish_token: str
    data_file: Path
    static_dir: Path

    @classmethod
    def from_env(cls):
        return cls(os.environ.get('WHOP_APP_ID', ''),
                   os.environ.get('WHOP_RESOURCE_ID', PRODUCT_ID),
                   os.environ.get('WHOP_API_KEY', ''),
                   os.environ.get('PRIVATE_PUBLISH_TOKEN', ''),
                   Path(os.environ.get('PRIVATE_DATA_FILE', '/var/lib/wraith/picks.json')),
                   Path(os.environ.get('PRIVATE_STATIC_DIR', 'site/dist')))

    def validate(self):
        if not re.fullmatch(r'app_[A-Za-z0-9]+', self.app_id):
            raise ValueError('Configure the ID of your own Whop app.')
        # The built-in Website Embed app is not an app this service owns.
        if self.app_id == 'app_OtuSFX6rkPguRs':
            raise ValueError('Create your own Whop app; do not use Website Embed app ID.')
        if self.resource_id != PRODUCT_ID:
            raise ValueError('This gateway is restricted to the WraithSznBets dashboard product.')
        if not self.api_key or len(self.publish_token) < 40:
            raise ValueError('Set the server API key and a random 40+ character publish token.')
        root = self.static_dir.resolve()
        if root in self.data_file.resolve().parents or root == self.data_file.resolve():
            raise ValueError('Private pick data cannot be inside the static directory.')
        if (root / 'data').exists():
            raise ValueError('Refusing a build containing public data. Use npm run build:private.')
        if not (root / 'index.html').is_file():
            raise ValueError('Build the private frontend before starting the server.')


def json_bytes(raw):
    def invalid(_):
        raise ValueError('Non-finite JSON value')
    return json.loads(raw, parse_constant=invalid)


def publication_time(payload):
    if not isinstance(payload, dict) or payload.get('sample'):
        raise ValueError('Only real exported board publications are accepted.')
    board = payload.get('board')
    if not isinstance(board, dict) or not isinstance(board.get('games'), list):
        raise ValueError('Missing board games.')
    datetime.strptime(board.get('date', ''), '%Y-%m-%d')
    at = datetime.fromisoformat(payload.get('generatedAt', '').replace('Z', '+00:00'))
    if at.tzinfo is None or at > datetime.now(timezone.utc):
        raise ValueError('Invalid publication timestamp.')
    return at


def create_app(settings=None, *, verifier=None, access_client=None):
    settings = settings or Settings.from_env()
    settings.validate()
    verify = verifier or verify_user_token
    client = access_client or Whop(token=settings.api_key, api_version_date='2026-08-21')
    app = FastAPI(docs_url=None, redoc_url=None, openapi_url=None)
    lock = threading.RLock()
    rate = {}
    root = settings.static_dir.resolve()

    @app.middleware('http')
    async def headers(request, call_next):
        response = await call_next(request)
        response.headers['X-Content-Type-Options'] = 'nosniff'
        response.headers['Referrer-Policy'] = 'no-referrer'
        response.headers['Content-Security-Policy'] = "frame-ancestors https://whop.com https://*.whop.com"
        if request.url.path.startswith(('/api/', '/internal/')):
            response.headers['Cache-Control'] = 'private, no-store, max-age=0'
            response.headers['Vary'] = 'x-whop-user-token'
        return response

    @app.get('/healthz')
    def health():
        return {'ok': True}

    @app.get('/api/picks')
    def picks(request: Request):
        token = request.headers.get('x-whop-user-token', '')
        if not token or len(token) > 16384:
            return JSONResponse({'error': 'authentication_required', 'joinUrl': STORE_URL}, status_code=401)
        try:
            identity = verify(token, app_id=settings.app_id)
            # The official SDK verifies signature/issuer/audience. Additionally
            # require an expiry claim; decoding below trusts nothing until that
            # signature check has completed for this exact token.
            jwt.decode(token, options={'verify_signature': False, 'verify_exp': True, 'require': ['exp']})
            uid = identity.user_id
            if not isinstance(uid, str) or not re.fullmatch(r'user_[A-Za-z0-9]+', uid):
                raise ValueError('Invalid subject')
        except Exception:
            return JSONResponse({'error': 'authentication_required', 'joinUrl': STORE_URL}, status_code=401)
        now = time.monotonic()
        with lock:
            if len(rate) > 10000:
                expired = [key for key, (start, _) in rate.items() if now-start > 60]
                for key in expired:
                    del rate[key]
                if len(rate) > 10000:
                    return JSONResponse({'error': 'try_again'}, status_code=429)
            start, count = rate.get(uid, (now, 0))
            if now-start >= 60:
                start, count = now, 0
            if count >= 30:
                return JSONResponse({'error': 'try_again'}, status_code=429)
            rate[uid] = (start, count+1)
        try:
            access = client.users.check_access(id=uid, resource_id=settings.resource_id,
                                             request_options={'timeout': 8, 'max_retries': 0})
        except Exception:
            return JSONResponse({'error': 'access_check_unavailable'}, status_code=503)
        if access.has_access is not True or access.access_level not in ('customer', 'admin'):
            return JSONResponse({'error': 'membership_required', 'joinUrl': STORE_URL}, status_code=403)
        try:
            with lock:
                body = settings.data_file.read_bytes()
            publication_time(json_bytes(body))
        except Exception:
            return JSONResponse({'error': 'board_unavailable'}, status_code=503)
        # No membership result is cached: every fetch above checks Whop again.
        return Response(body, media_type='application/json')

    @app.post('/internal/publish')
    async def publish(request: Request):
        expected = 'Bearer ' + settings.publish_token
        if not hmac.compare_digest(request.headers.get('authorization', '').encode(), expected.encode()):
            return JSONResponse({'error': 'unauthorized'}, status_code=401)
        if request.headers.get('content-type', '').split(';')[0].strip() != 'application/json':
            return JSONResponse({'error': 'json_required'}, status_code=415)
        body = bytearray()
        async for chunk in request.stream():
            body.extend(chunk)
            if len(body) > MAX_BYTES:
                return JSONResponse({'error': 'too_large'}, status_code=413)
        try:
            incoming = publication_time(json_bytes(body))
        except Exception:
            return JSONResponse({'error': 'invalid_publication'}, status_code=400)
        with lock:
            if settings.data_file.exists():
                try:
                    current = publication_time(json_bytes(settings.data_file.read_bytes()))
                except Exception:
                    return JSONResponse({'error': 'stored_board_invalid'}, status_code=409)
                if incoming < current:
                    return JSONResponse({'error': 'older_publication'}, status_code=409)
            settings.data_file.parent.mkdir(parents=True, exist_ok=True)
            fd, temp = tempfile.mkstemp(prefix='.picks-', dir=settings.data_file.parent)
            try:
                with os.fdopen(fd, 'wb') as stream:
                    stream.write(body)
                    stream.flush()
                    os.fsync(stream.fileno())
                os.chmod(temp, 0o600)
                os.replace(temp, settings.data_file)
            finally:
                if os.path.exists(temp):
                    os.unlink(temp)
        return {'ok': True, 'sha256': hashlib.sha256(body).hexdigest()}

    @app.get('/{path:path}')
    def static(path: str):
        # Explicit allowlist: never mount the repository, data folder, private
        # volume or a legacy public picks file. Query strings do not affect it.
        if path in ('', 'index.html') or re.fullmatch(r'experiences/exp_[A-Za-z0-9]+/?', path):
            return FileResponse(root/'index.html', headers={'Cache-Control': 'no-store'})
        if path == 'manifest.webmanifest' or path.startswith(('assets/', 'brand/')):
            target = (root/path).resolve()
            if target.is_relative_to(root) and target.is_file() and not target.name.startswith('.'):
                return FileResponse(target)
        return JSONResponse({'error': 'not_found'}, status_code=404)
    return app

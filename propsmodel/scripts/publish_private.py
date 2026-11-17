"""Send an export to the private gateway; never print a credential or response body."""
import hashlib
import json
import os
from pathlib import Path
import sys
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def publish():
    origin = os.environ.get('PRIVATE_BOARD_URL', '').rstrip('/')
    target = urllib.parse.urlsplit(origin)
    token = os.environ.get('PRIVATE_PUBLISH_TOKEN', '')
    if (target.scheme != 'https' or not target.hostname or target.username or target.password
            or target.path or target.query or target.fragment or len(token) < 40):
        raise ValueError('Configure an HTTPS origin and a random 40+ character publication secret.')
    body = Path('private/picks.json').read_bytes()
    if len(body) > 25*1024*1024:
        raise ValueError('Publication exceeds gateway size limit.')
    payload = json.loads(body)
    if payload.get('sample') or not isinstance(payload.get('board'), dict):
        raise ValueError('Refusing sample or invalid board.')
    request = urllib.request.Request(origin+'/internal/publish', data=body, method='POST',
                                    headers={'Authorization': 'Bearer '+token, 'Content-Type': 'application/json'})
    opener = urllib.request.build_opener(NoRedirect())
    with opener.open(request, timeout=45) as response:
        result = json.load(response)
    if result.get('ok') is not True or result.get('sha256') != hashlib.sha256(body).hexdigest():
        raise ValueError('Private publication did not confirm the expected content.')
    print('Board delivered to the protected gateway.')


if __name__ == '__main__':
    try:
        publish()
    except Exception:
        print('Private publication failed. Check host configuration and private workflow secrets.', file=sys.stderr)
        sys.exit(1)

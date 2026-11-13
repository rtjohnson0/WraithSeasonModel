import json
from pathlib import Path
from types import SimpleNamespace
import tempfile
import time
import unittest

from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import serialization
from fastapi.testclient import TestClient
import httpx
import jwt
from whop_sdk import Whop
from whop_sdk.lib.verify_user_token import verify_user_token

from server.app import Settings, create_app, PRODUCT_ID


class GatewayTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.static = self.root/'dist'
        self.static.mkdir()
        (self.static/'index.html').write_text('<div id="root"></div>')
        self.settings = Settings('app_testowned', PRODUCT_ID, 'test-api-key', 'x'*48,
                                 self.root/'private/picks.json', self.static)
        self.key = ec.generate_private_key(ec.SECP256R1())
        self.public = self.key.public_key().public_bytes(serialization.Encoding.PEM,
                                                        serialization.PublicFormat.SubjectPublicKeyInfo).decode()
        self.requests = []
        self.allowed = True
        self.upstream_error = False
        def transport(request):
            self.requests.append(request)
            if self.upstream_error:
                return httpx.Response(500, json={'error': 'unavailable'})
            return httpx.Response(200, json={'has_access': self.allowed,
                                            'access_level': 'customer' if self.allowed else 'no_access'})
        self.http = httpx.Client(transport=httpx.MockTransport(transport), trust_env=False)
        self.addCleanup(self.http.close)
        whop = Whop(token='test-api-key', api_version_date='2026-08-21', httpx_client=self.http)
        def verifier(token, app_id):
            return verify_user_token(token, app_id=app_id, public_key=self.public)
        self.app = create_app(self.settings, verifier=verifier, access_client=whop)
        self.client = TestClient(self.app)
        self.addCleanup(self.client.close)
        self.payload = {'generatedAt': '2026-01-01T00:00:00Z',
                        'board': {'date': '2026-01-01', 'games': [], 'secretPick': 'test premium contents'}}

    def token(self, **changes):
        claims = {'sub': 'user_testmember', 'aud': 'app_testowned',
                  'iss': 'urn:whopcom:exp-proxy', 'exp': int(time.time())+120}
        claims.update(changes)
        return jwt.encode(claims, self.key, algorithm='ES256')

    def publish(self, body=None, authorized=True):
        headers = {'Authorization': 'Bearer '+('x'*48 if authorized else 'bad')}
        return self.client.post('/internal/publish', json=body or self.payload, headers=headers)

    def fetch(self, token=None):
        return self.client.get('/api/picks', headers={'x-whop-user-token': token or self.token()})

    def test_anonymous_and_forged_identity_never_read_picks(self):
        self.publish()
        for headers in ({}, {'x-whop-user-id': 'user_testmember'}, {'x-whop-user-token': 'fake'}):
            r = self.client.get('/api/picks?resource_id=prod_other&user_id=user_testmember', headers=headers)
            self.assertEqual(r.status_code, 401)
            self.assertNotIn('test premium contents', r.text)
        self.assertEqual(self.requests, [])

    def test_real_signature_expiry_issuer_and_app_audience_are_checked(self):
        self.publish()
        variants = [self.token(exp=int(time.time())-1), self.token(aud='app_someoneelse'),
                    self.token(iss='fake'), self.token(sub='not-a-whop-user')]
        claims = {'sub': 'user_testmember', 'aud': 'app_testowned', 'iss': 'urn:whopcom:exp-proxy'}
        variants.append(jwt.encode(claims, self.key, algorithm='ES256'))
        wrong_key = ec.generate_private_key(ec.SECP256R1())
        variants.append(jwt.encode({**claims, 'exp': int(time.time())+120}, wrong_key, algorithm='ES256'))
        for token in variants:
            with self.subTest(token_kind=token[:12]):
                self.assertEqual(self.fetch(token).status_code, 401)
        self.assertEqual(self.requests, [])

    def test_member_access_is_live_specific_and_not_cached(self):
        self.assertEqual(self.publish().status_code, 200)
        r = self.fetch()
        self.assertEqual(r.status_code, 200)
        self.assertIn('test premium contents', r.text)
        self.assertIn('no-store', r.headers['cache-control'])
        req = self.requests[-1]
        self.assertEqual(req.url.path, '/api/v1/users/user_testmember/access/'+PRODUCT_ID)
        self.assertEqual(req.headers['authorization'], 'Bearer test-api-key')
        self.allowed = False
        r = self.fetch()
        self.assertEqual(r.status_code, 403)
        self.assertNotIn('test premium contents', r.text)
        self.assertEqual(len(self.requests), 2)

    def test_access_service_errors_fail_closed(self):
        self.publish()
        self.upstream_error = True
        r = self.fetch()
        self.assertEqual(r.status_code, 503)
        self.assertNotIn('test premium contents', r.text)

    def test_old_public_files_and_path_traversal_are_not_served(self):
        self.publish()
        for path in ['/data/picks.json', '/private/picks.json', '/.env', '/.git/config',
                     '/assets/%2e%2e/private/picks.json', '/api/not-real']:
            r = self.client.get(path)
            self.assertEqual(r.status_code, 404, path)
            self.assertNotIn('test premium contents', r.text)
        self.assertEqual(self.client.get('/experiences/exp_example').status_code, 200)

    def test_publish_auth_validation_and_rollback_protection(self):
        self.assertEqual(self.publish(authorized=False).status_code, 401)
        self.assertFalse(self.settings.data_file.exists())
        self.assertEqual(self.publish({'sample': True}).status_code, 400)
        self.assertEqual(self.publish().status_code, 200)
        old = {**self.payload, 'generatedAt': '2025-12-31T00:00:00Z'}
        self.assertEqual(self.publish(old).status_code, 409)
        self.assertEqual(json.loads(self.settings.data_file.read_bytes()), self.payload)
        self.assertIn('no-store', self.publish().headers['cache-control'])

    def test_startup_refuses_public_premium_build_or_wrong_product(self):
        (self.static/'data').mkdir()
        with self.assertRaises(ValueError):
            self.settings.validate()
        (self.static/'data').rmdir()
        with self.assertRaises(ValueError):
            Settings(self.settings.app_id, 'prod_another', 'key', 'x'*48,
                     self.settings.data_file, self.static).validate()
        with self.assertRaises(ValueError):
            Settings('app_OtuSFX6rkPguRs', PRODUCT_ID, 'key', 'x'*48,
                     self.settings.data_file, self.static).validate()

    def test_rate_limit_cannot_be_used_to_bypass_access_check(self):
        self.publish()
        for _ in range(30):
            self.assertEqual(self.fetch().status_code, 200)
        self.assertEqual(self.fetch().status_code, 429)
        self.assertEqual(len(self.requests), 30)


if __name__ == '__main__':
    unittest.main()

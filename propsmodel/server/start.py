import os
import uvicorn

if __name__ == '__main__':
    # One worker serializes publication and keeps the per-user rate limit local.
    uvicorn.run('server.app:create_app', factory=True, host='0.0.0.0',
                port=int(os.environ.get('PORT', '8000')), workers=1,
                proxy_headers=False, access_log=False)

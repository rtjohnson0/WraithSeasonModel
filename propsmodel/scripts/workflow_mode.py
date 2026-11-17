"""Select DST-correct Eastern run slots, using cron rather than queue start hour."""
import os
from datetime import datetime
from zoneinfo import ZoneInfo


def select(event, requested, schedule, branch, default_branch, now=None):
    if branch != default_branch:
        raise ValueError('Publishing is restricted to the default branch.')
    if event == 'push':
        return 'site', True
    if event == 'workflow_dispatch':
        if requested not in {'bootstrap', 'daily', 'pretip', 'site'}:
            raise ValueError('Unknown workflow mode')
        return requested, True
    now = now or datetime.now(ZoneInfo('America/New_York'))
    hour = int(schedule.split()[1])
    offset = int(now.utcoffset().total_seconds() // 3600)
    target = {13:9,14:9,19:15,20:15,22:18,23:18,0:20,1:20}.get(hour)
    if target is None:
        raise ValueError('Unexpected schedule')
    # The cron expression identifies the intended slot even after a long queue delay.
    return ('daily' if target == 9 else 'pretip'), (hour + offset) % 24 == target



if __name__ == '__main__':
    mode, active = select(os.environ['EVENT_NAME'], os.environ.get('REQUESTED_MODE', ''),
                          os.environ.get('EVENT_SCHEDULE', ''), os.environ['RUN_BRANCH'],
                          os.environ['DEFAULT_BRANCH'])
    # Bootstrap downloads inputs; it intentionally does not deploy an untrained site.
    publish = active and mode != 'bootstrap'
    with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as out:
        out.write(f'mode={mode}\nactive={str(active).lower()}\npublish={str(publish).lower()}\n')

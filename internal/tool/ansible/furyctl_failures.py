# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Writes the last failure of each host that the final stats count as failed or unreachable to the
# file in FURYCTL_ANSIBLE_FAILURES, so furyctl can tell why ansible failed. As an aggregate callback it
# loads next to the stdout callback, without callbacks_enabled.
import json
import os

from ansible.plugins.callback import CallbackBase


def _attr(result, name):
    # ansible-core 2.19 added result, host and task. Older versions have only _result, _host and _task.
    return getattr(result, name) if hasattr(result, name) else getattr(result, '_' + name)


class CallbackModule(CallbackBase):
    CALLBACK_VERSION = 2.0
    CALLBACK_TYPE = 'aggregate'
    CALLBACK_NAME = 'furyctl_failures'
    CALLBACK_NEEDS_ENABLED = False

    def __init__(self):
        super().__init__()
        self._play = ''
        self._last = {}

    def v2_playbook_on_play_start(self, play):
        self._play = play.get_name()

    def _record(self, unreachable, result):
        res = _attr(result, 'result')
        msg = res.get('msg', '')
        rc = res.get('rc')
        stderr = res.get('stderr', '')
        self._last[_attr(result, 'host').get_name()] = {
            'unreachable': unreachable,
            'play': self._play,
            'task': _attr(result, 'task').get_name(),
            'msg': msg if isinstance(msg, str) else json.dumps(msg),
            'rc': rc if isinstance(rc, int) else None,
            'stderr': stderr if isinstance(stderr, str) else '',
        }

    def v2_runner_on_failed(self, result, ignore_errors=False):
        if not ignore_errors:
            self._record(False, result)

    def v2_runner_on_unreachable(self, result):
        if not _attr(result, 'task').ignore_unreachable:
            self._record(True, result)

    def v2_playbook_on_stats(self, stats):
        path = os.environ.get('FURYCTL_ANSIBLE_FAILURES')
        if not path:
            return

        # The stats leave out the failures that a rescue handled.
        failures = []
        for host, failure in self._last.items():
            s = stats.summarize(host)
            if s['failures'] or s['unreachable']:
                failures.append(dict(failure, host=host))

        with open(path, 'w') as f:
            json.dump(failures, f)

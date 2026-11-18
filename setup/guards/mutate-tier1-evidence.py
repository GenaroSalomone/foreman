#!/usr/bin/env python3
"""Prove tier-1 evidence checks with isolated, post-commit executable mutants."""
import io
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile

repo = Path(__file__).resolve().parents[2]
ref = sys.argv[1] if len(sys.argv) == 2 else 'HEAD'
sha = subprocess.check_output(['git', '-C', str(repo), 'rev-parse', ref + '^{commit}'], text=True).strip()
archive = subprocess.check_output(['git', '-C', str(repo), 'archive', sha, 'bin', 'setup/tests', 'setup/guards', 'setup/fixtures'])
env = {k: v for k, v in os.environ.items() if not k.startswith(('HW_', 'GIT_'))}

def once(s, old, new):
    assert s.count(old) == 1, ('anchor count', old, s.count(old))
    return s.replace(old, new, 1)

mutations = [
    ('wait-ack', 'bin/herdr-rpc', '98-wait-agent-requires-a-matching-event.sh',
     lambda s: re.sub(r'            # A successful RPC exchange.*?(?=        except RpcError as e:)', '            results.put(("ok", status, res))\n', s, count=1, flags=re.S),
     'generic acknowledgment is not a match'),
    ('wait-pane', 'bin/herdr-rpc', '98-wait-agent-requires-a-matching-event.sh',
     lambda s: once(s, 'data.get("pane_id") == pane_id', 'True'), 'wrong pane is not a match'),
    ('wait-status', 'bin/herdr-rpc', '98-wait-agent-requires-a-matching-event.sh',
     lambda s: once(s, 'data.get("agent_status") == status', 'True'), 'wrong status is not a match'),
    ('ask-caller-record-claim', 'bin/ask-invoker', '99-unverified-publication-through-callers.sh',
     lambda s: once(s, 'PUBLICATION_NOTE="$(invoker_publication_note "$PUB_RC")"', 'PUBLICATION_NOTE="$(invoker_publication_note "$PUB_RC")"\n[ "$PUB_RC" = 0 ] || PUBLICATION_NOTE="The question remains recorded on this pane."'),
     'The question remains recorded on this pane.'),
    ('done-caller-record-claim', 'bin/done-invoker', '99-unverified-publication-through-callers.sh',
     lambda s: once(s, 'PUBLICATION_NOTE="$(invoker_publication_note "$PUB_RC")"', 'PUBLICATION_NOTE="$(invoker_publication_note "$PUB_RC")"\n[ "$PUB_RC" = 0 ] || PUBLICATION_NOTE="The completion remains recorded on this pane."'),
     'The completion remains recorded on this pane.'),
]
with tempfile.TemporaryDirectory(prefix='tier1-mutants-') as temporary:
    for label, target, subject, mutate, witness in [('baseline', None, None, None, None)] + mutations:
        root = Path(temporary) / label
        root.mkdir()
        with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
            tar.extractall(root)
        if mutate:
            path = root / target
            source = path.read_text()
            changed = mutate(source)
            assert changed != source, ('no mutation', label)
            path.write_text(changed)
        for name in [subject] if subject else sorted({m[2] for m in mutations}):
            run = subprocess.run(['bash', str(root / 'setup/tests' / name)], cwd=root, env=env,
                                 stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            assert (run.returncode != 0 and witness in run.stdout) if mutate else run.returncode == 0, (label, run.returncode, run.stdout)
        print(('KILLED ' if mutate else 'PASS ') + label + ' @ ' + sha, flush=True)

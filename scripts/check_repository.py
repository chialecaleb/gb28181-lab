"""Check foundational repository invariants, not live GitHub state."""
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def main():
    errors = []
    for name in ('AGENTS.md', 'USER-ACTIONS.md', 'docs/README.md', 'docs/delivery/status.md'):
        if not (ROOT / name).is_file():
            errors.append('Missing entry: ' + name)
    rules = json.loads((ROOT / '.github/rulesets/main.json').read_text(encoding='utf-8'))
    by_type = {r['type']: r for r in rules['rules']}
    for kind in ('deletion', 'non_fast_forward', 'required_linear_history', 'pull_request', 'required_status_checks'):
        if kind not in by_type:
            errors.append('Missing protection: ' + kind)
    if rules['bypass_actors'] or rules['enforcement'] != 'active':
        errors.append('Protection must be active with no bypass actors')
    if rules['conditions']['ref_name']['include'] != ['refs/heads/main']:
        errors.append('Protection must target main')
    check = by_type.get('required_status_checks', {}).get('parameters', {}).get('required_status_checks', [])
    if check != [{'context': 'repository-quality', 'integration_id': 15368}]:
        errors.append('Required GitHub Actions check mismatch')
    workflow = (ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8')
    if '  repository-quality:' not in workflow or 'python scripts/verify.py' not in workflow:
        errors.append('CI does not expose the documented gate')
    tracked = subprocess.check_output(['git', 'ls-files', '--stage', '-z'], cwd=ROOT).decode().split('\0')
    if any(line.startswith('160000 ') for line in tracked):
        errors.append('Unexpected submodule/gitlink')
    if (ROOT / 'repos.json').exists():
        if any('\tgb28181-hub/' in x or '\tgb28181-lab/' in x for x in tracked):
            errors.append('Workspace must not track product files')
    if errors:
        print('\n'.join(errors))
        return 1
    print('PASS: repository boundaries and expected protection/CI contract')
    return 0

if __name__ == '__main__':
    sys.exit(main())

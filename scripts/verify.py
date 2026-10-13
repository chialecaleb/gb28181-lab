"""Run the actual repository gate, using Python standard library only."""
import py_compile
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def main():
    for script in sorted((ROOT / 'scripts').glob('*.py')):
        py_compile.compile(str(script), doraise=True)
    commands = [[sys.executable, 'scripts/check_docs.py'],
                [sys.executable, 'scripts/check_repository.py'],
                [sys.executable, '-m', 'unittest', 'discover', '-s', 'scripts', '-p', 'test_*.py'],
                ['git', 'diff', '--check'], ['git', 'diff', '--cached', '--check']]
    if (ROOT / 'scripts/check_sources.py').exists():
        commands.insert(2, [sys.executable, 'scripts/check_sources.py'])
    for command in commands:
        print('RUN:', ' '.join(command), flush=True)
        result = subprocess.run(command, cwd=ROOT)
        if result.returncode:
            return result.returncode
    print('PASS: repository foundation gate; application acceptance is not covered')
    return 0

if __name__ == '__main__':
    sys.exit(main())

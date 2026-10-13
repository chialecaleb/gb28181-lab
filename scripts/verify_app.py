"""Application gate, independent of private .env.local and external platforms."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def run(args, cwd=ROOT):
    executable = shutil.which(args[0])
    if not executable:
        raise RuntimeError('missing tool: ' + args[0])
    print('+ ' + ' '.join(args), flush=True)
    subprocess.run([executable, *args[1:]], cwd=cwd, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--race', action='store_true')
    parser.add_argument('--e2e', action='store_true')
    args = parser.parse_args()
    files = [str(p.relative_to(ROOT)) for folder in ('internal', 'cmd') for p in (ROOT / folder).rglob('*.go')]
    files.append('web/assets.go')
    formatting = subprocess.check_output(['gofmt', '-l', *files], cwd=ROOT, text=True)
    if formatting.strip():
        raise RuntimeError('Go formatting required: ' + formatting)
    run(['pnpm', 'install', '--frozen-lockfile'], ROOT / 'web')
    run(['pnpm', 'exec', 'prettier', '--check', 'src', 'e2e', 'package.json', 'tsconfig.json', 'vite.config.ts', 'playwright.config.ts', 'index.html'], ROOT / 'web')
    run(['pnpm', 'test'], ROOT / 'web')
    run(['pnpm', 'build'], ROOT / 'web')
    run(['go', 'mod', 'verify'])
    run(['go', 'test', *(['-race'] if args.race else []), './internal/...', '-timeout', '180s'])
    run(['go', 'vet', './internal/...', './cmd/...', './web'])
    suffix = '.exe' if os.name == 'nt' else ''
    for name in ('gb28181-lab-server', 'gb28181-lab'):
        run(['go', 'build', '-o', 'bin/' + name + suffix, './cmd/' + name])
    if os.name == 'nt':
        run(['go', 'build', '-tags', 'production', '-ldflags', '-H=windowsgui', '-o', 'bin/gb28181-lab-desktop.exe', './cmd/gb28181-lab-desktop'])
    if args.e2e:
        run(['pnpm', 'e2e'], ROOT / 'web')
    print('Application gate passed; external interoperability and full standard acceptance are separate.')


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)

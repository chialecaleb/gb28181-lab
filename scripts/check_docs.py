"""Check tracked documentation using only the Python standard library."""
import json
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[1]
SOURCES = ('docs/standards/GBT28181-2022/', 'docs/standards/GB35114-2017/')


def anchors(text):
    result = set(re.findall(r'<a\s+id=["\']([^"\']+)', text))
    counts = {}
    for heading in re.findall(r'^#{1,6}\s+(.+)$', text, re.M):
        slug = re.sub(r'[^\w\- ]', '', heading.lower()).replace(' ', '-')
        n = counts.get(slug, 0)
        counts[slug] = n + 1
        result.add(slug if n == 0 else f'{slug}-{n}')
    return result


def main():
    tracked = subprocess.check_output(
        ['git', 'ls-files', '-z'], cwd=ROOT
    ).decode('utf-8').split('\0')
    markdown = {p for p in tracked if p.endswith('.md')}
    own = {p for p in markdown if not p.startswith(SOURCES)}
    errors = []
    registry = json.loads((ROOT / 'docs/governance/document-registry.json').read_text(encoding='utf-8'))
    entries = registry['documents']
    paths = [e['path'] for e in entries]
    if len(paths) != len(set(paths)):
        errors.append('Duplicate registry paths')
    for p in sorted(own - set(paths)):
        errors.append(f'Unregistered Markdown: {p}')
    for p in sorted(set(paths) - own):
        errors.append(f'Registry path is not tracked owned Markdown: {p}')
    for entry in entries:
        for key in ('path', 'kind', 'status', 'owner', 'scope', 'reviewed', 'basis'):
            if not entry.get(key):
                errors.append(f'Missing {key}: {entry.get("path")}')
        if entry.get('status') not in ('active', 'draft', 'reference', 'superseded'):
            errors.append(f'Invalid status: {entry["path"]}')
        if not re.fullmatch(r'\d{4}-\d{2}-\d{2}', entry.get('reviewed', '')):
            errors.append(f'Invalid reviewed date: {entry["path"]}')
    checked_links = 0
    for p in sorted(markdown):
        file = ROOT / p
        if not file.exists():
            errors.append(f'Tracked file missing: {p}')
            continue
        text = file.read_text(encoding='utf-8-sig')
        if p in own and re.search(r'(?i)\b[A-Z]:[\\/]|file://|/Users/|/home/[^\s/]+/', text):
            errors.append(f'Machine-specific absolute path: {p}')
        # Ignore fenced examples; inspect inline links and images in real text.
        prose = re.sub(r'^```[^\n]*\n.*?^```\s*$', '', text, flags=re.M | re.S)
        pattern = r'!?\[[^\]\n]*\]\(([^)\n]+)\)' if p in own else r'!\[[^\]\n]*\]\(([^)\n]+)\)'
        for match in re.finditer(pattern, prose):
            target = match.group(1).strip()
            target = target[1:target.index('>')] if target.startswith('<') else target.split(' "')[0]
            if re.match(r'^[a-zA-Z][\w+.-]*:', target):
                continue
            path, _, fragment = unquote(target).partition('#')
            dest = (file.parent / path).resolve() if path else file
            if not dest.is_relative_to(ROOT):
                errors.append(f'Link escapes repository: {p}: {target}')
                continue
            checked_links += 1
            if not dest.exists():
                errors.append(f'Broken link: {p}: {target}')
            elif fragment and dest.suffix == '.md':
                if fragment not in anchors(dest.read_text(encoding='utf-8-sig')):
                    errors.append(f'Broken anchor: {p}: {target}')
    if errors:
        print('\n'.join(errors))
        return 1
    print(f'PASS: {len(own)} registered documents, {len(markdown) - len(own)} source documents, {checked_links} local links')
    return 0


if __name__ == '__main__':
    sys.exit(main())

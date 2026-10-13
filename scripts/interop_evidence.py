"""Independent byte-level checks for controlled external signaling evidence.

This uses Python's XML/GB18030 support, not the Go encoder or SIP implementation.
No private identities, headers, addresses or credentials are returned.
"""
from datetime import datetime
import hashlib
import json
import re
import xml.etree.ElementTree as ET


class EvidenceError(ValueError):
    pass


def read_messages(directory):
    evidence = json.loads((directory / 'evidence.json').read_text(encoding='utf-8'))
    trace = evidence['traces']
    records = trace['records']
    files = sorted(directory.glob('*.sip'))
    if trace.get('dropped') or len(files) != len(records) or not files:
        raise EvidenceError('wire evidence is incomplete')
    messages = []
    aliases = {'v': 'via', 'i': 'call-id', 'f': 'from', 't': 'to', 'l': 'content-length'}
    for path, record in zip(files, records):
        raw = path.read_bytes()
        if (record.get('truncated') or len(raw) != record['length']
                or hashlib.sha256(raw).hexdigest() != record['sha256']):
            raise EvidenceError('wire evidence fingerprint mismatch')
        head, separator, body = raw.partition(b'\r\n\r\n')
        if not separator:
            raise EvidenceError('wire frame has no header terminator')
        lines = head.decode('gb18030').split('\r\n')
        headers = {}
        for line in lines[1:]:
            name, separator, value = line.partition(':')
            name = aliases.get(name.lower(), name.lower())
            if not separator or name in headers:
                raise EvidenceError('invalid or duplicate SIP header')
            headers[name] = value.strip()
        if int(headers.get('content-length', '-1')) != len(body):
            raise EvidenceError('wire Content-Length mismatch')
        branch = re.search(r'(?:^|;)branch=([^;\s]+)', headers.get('via', ''))
        identities = [re.search(r'sips?:([^@>;\s]+)@', headers.get(name, ''))
                      for name in ('from', 'to')]
        if not branch or not headers.get('call-id') or not headers.get('cseq') or not all(identities):
            raise EvidenceError('wire transaction fields missing')
        message = {'headers': headers, 'body': body, 'direction': record['direction'],
                   'transport': record.get('transport', 'unknown'),
                   'protocolMode': record.get('protocolMode', 'standard'),
                   'compatibility': [rule for rule in record.get('compatibility', [])
                                     if rule in ('COMPAT-001', 'COMPAT-002')],
                   'at': datetime.fromisoformat(record['at'].replace('Z', '+00:00')),
                   'key': (headers['call-id'], headers['cseq'], branch.group(1),
                           *(identity.group(1) for identity in identities)),
                   'error': record.get('error'), 'xml': None}
        start = lines[0].split(' ')
        if start[0] == 'SIP/2.0':
            message['status'] = int(start[1])
        else:
            if len(start) != 3 or start[2] != 'SIP/2.0':
                raise EvidenceError('invalid request line')
            message.update(method=start[0], uri=start[1])
            if body:
                if re.search(br'<!\s*(?:DOCTYPE|ENTITY)', body, re.I):
                    raise EvidenceError('XML declarations are not allowed')
                message['xml'] = ET.fromstring(body.decode('gb18030'))
                if 'COMPAT-002' in message['compatibility'] and (
                        not body.isascii() or message['xml'].tag != 'Query'
                        or message['xml'].findtext('CmdType') not in ('DeviceInfo', 'Catalog')):
                    raise EvidenceError('legacy query exceeded its declared compatibility scope')
        messages.append(message)
    return messages


def assess_messages(messages, serial, channel):
    # De-duplicate transactions, never count UDP retransmissions as extra cycles.
    requests = {}
    responses = {}
    for message in messages:
        if message.get('error'):
            continue
        key = (message['key'], message['direction'])
        if 'method' in message:
            requests.setdefault(key, message)
        elif message['status'] >= 200:
            responses.setdefault(key, []).append(message)

    def replies(request):
        direction = 'in' if request['direction'] == 'out' else 'out'
        return responses.get((request['key'], direction), [])

    def acknowledged(request, code=200):
        return any(reply['status'] == code and not reply['body'] for reply in replies(request))

    outgoing = [r for r in requests.values() if r['direction'] == 'out']
    registrations = [r for r in outgoing if r['method'] == 'REGISTER']
    accepted = [r for r in registrations if r['headers'].get('expires') != '0'
                and r['headers'].get('authorization') and acknowledged(r)]
    deregistered = any(r['headers'].get('expires') == '0'
                       and r['headers'].get('authorization') and acknowledged(r) for r in registrations)
    challenged = any(not r['headers'].get('authorization') and any(
        reply['status'] == 401 and not reply['body'] and reply['headers'].get('www-authenticate')
        for reply in replies(r)) for r in registrations)

    def command(message, root, name):
        xml = message.get('xml')
        return (message.get('method') == 'MESSAGE' and xml is not None and xml.tag == root
                and xml.findtext('CmdType') == name and xml.findtext('DeviceID') == serial)

    heartbeats = sorted((r for r in outgoing if command(r, 'Notify', 'Keepalive')
                         and r['xml'].findtext('Status') == 'OK' and acknowledged(r)),
                        key=lambda r: r['at'])
    registration_time = min((reply['at'] for r in accepted for reply in replies(r)
                             if reply['status'] == 200), default=None)
    times = ([registration_time] if registration_time else []) + [r['at'] for r in heartbeats]
    intervals = [(b - a).total_seconds() for a, b in zip(times, times[1:])]
    normal_heartbeats = (len(heartbeats) >= 3 and registration_time is not None
                         and len(intervals) >= 3 and all(59 <= x <= 75 for x in intervals[:3]))

    business = {}
    for name in ('DeviceInfo', 'Catalog'):
        complete = False
        queries = [r for r in requests.values() if r['direction'] == 'in'
                   and command(r, 'Query', name) and acknowledged(r)]
        for query in queries:
            sn = query['xml'].findtext('SN')
            if not sn or not re.fullmatch(r'[0-9]+', sn) or not int(sn):
                continue
            parts = [r for r in outgoing if command(r, 'Response', name)
                     and r['xml'].findtext('SN') == sn and r['key'] != query['key']]
            if not parts or not all(acknowledged(r) for r in parts):
                continue
            query_ack = min(reply['at'] for reply in replies(query) if reply['status'] == 200)
            if any(r['at'] < query_ack for r in parts):
                continue
            if name == 'DeviceInfo':
                complete = (len(parts) == 1 and parts[0]['xml'].findtext('Result') == 'OK'
                            and parts[0]['xml'].findtext('Channel') == '1')
            else:
                totals = {p['xml'].findtext('SumNum') for p in parts}
                items = [item.findtext('DeviceID') for p in parts
                         for item in p['xml'].findall('./DeviceList/Item')]
                counts_match = all(
                    p['xml'].find('DeviceList') is not None
                    and p['xml'].find('DeviceList').get('Num') ==
                    str(len(p['xml'].findall('./DeviceList/Item'))) for p in parts)
                complete = (totals == {'2'} and counts_match and len(items) == 2
                            and set(items) == {serial, channel})
            if complete:
                break
        business[name.lower()] = complete

    checks = {'registration': bool(accepted), 'refresh': len(accepted) >= 2, 'digestChallenge': challenged,
              'deregistration': deregistered, 'normalHeartbeatCycles': normal_heartbeats,
              **business}
    operations = []
    for request in sorted(requests.values(), key=lambda r: r['at']):
        xml = request.get('xml')
        kind = request['method'] if request['method'] in ('REGISTER', 'MESSAGE') else 'unsupported'
        if kind == 'REGISTER' and request['headers'].get('expires') == '0':
            kind = 'unregister'
        if xml is not None:
            cmd = xml.findtext('CmdType')
            kind = cmd if cmd in ('DeviceInfo', 'Catalog', 'Keepalive') else 'unsupported'
        operations.append({'sequence': len(operations) + 1, 'kind': kind,
                           'root': xml.tag if xml is not None and xml.tag in ('Query', 'Response', 'Notify') else '',
                           'direction': request['direction'],
                           'transport': request.get('transport', 'unknown'),
                           'protocolMode': request.get('protocolMode', 'standard'),
                           'authenticated': bool(request['headers'].get('authorization')),
                           'sipStatuses': sorted({reply['status'] for reply in replies(request)}),
                           'compatibility': sorted(set(request.get('compatibility', [])) | (
                               {'COMPAT-001'} if request['method'] == 'REGISTER'
                               and request['direction'] == 'out' and '@' in request.get('uri', '') else set()))})
    return {'passed': all(checks.values()), **checks, 'operations': operations,
            'heartbeatAcknowledged': len(heartbeats),
            'heartbeatIntervalsSeconds': [round(x, 3) for x in intervals],
            'registerRequestUserinfo': any('@' in r.get('uri', '') for r in registrations)}


def inspect_evidence(directory, serial, channel):
    return assess_messages(read_messages(directory), serial, channel)

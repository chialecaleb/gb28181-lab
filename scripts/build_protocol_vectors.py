"""Independent, reviewable M0 fixtures; never imports the Go implementation.

Manual XML field order follows the reviewed A.2 schema. Python's codec and
hashlib are independent byte/digest oracles. Commit fixtures, do not regenerate
expected values during Go tests. This script uses only synthetic identities.
"""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEVICE = '34020000001180000001'
PLATFORM = '34020000002000000001'


def main():
    xml = {
        'keepalive': f'<Notify>\r\n<CmdType>Keepalive</CmdType>\r\n<SN>1</SN>\r\n<DeviceID>{DEVICE}</DeviceID>\r\n<Status>OK</Status>\r\n<Info></Info>\r\n</Notify>\r\n',
        'info-query': f'<Query>\r\n<CmdType>DeviceInfo</CmdType>\r\n<SN>2</SN>\r\n<DeviceID>{DEVICE}</DeviceID>\r\n</Query>\r\n',
        'info-response': f'<Response>\r\n<CmdType>DeviceInfo</CmdType>\r\n<SN>2</SN>\r\n<DeviceID>{DEVICE}</DeviceID>\r\n<DeviceName>实验室 &amp; 摄像机𠀀</DeviceName>\r\n<Result>OK</Result>\r\n<Manufacturer>Lab</Manufacturer>\r\n<Model>M0</Model>\r\n<Firmware>0.1</Firmware>\r\n<Channel>1</Channel>\r\n</Response>\r\n',
        'catalog-empty': f'<Response>\r\n<CmdType>Catalog</CmdType>\r\n<SN>3</SN>\r\n<DeviceID>{DEVICE}</DeviceID>\r\n<SumNum>0</SumNum>\r\n</Response>\r\n',
    }
    vectors = []
    for name, body in xml.items():
        text = '<?xml version="1.0" encoding="GB18030"?>\r\n' + body
        data = text.encode('gb18030')
        vectors.append(dict(name=name, text=text, hex=data.hex(), length=len(data), sha256=hashlib.sha256(data).hexdigest()))
    md5 = lambda s: hashlib.md5(s.encode('ascii')).hexdigest()
    a1 = md5(f'{DEVICE}:3402000000:synthetic-secret')
    a2 = md5('REGISTER:sip:127.0.0.1:15062')
    digest = dict(username=DEVICE, realm='3402000000', password='synthetic-secret', method='REGISTER', uri='sip:127.0.0.1:15062', nonce='0123456789abcdef', cnonce='abcdef0123456789', nc='00000001', ha1=a1, ha2=a2,
                  legacy=md5(f'{a1}:0123456789abcdef:{a2}'), auth=md5(f'{a1}:0123456789abcdef:00000001:abcdef0123456789:auth:{a2}'))
    obj = dict(schemaVersion='1.0', source='m0-contract.json; manual XML + Python gb18030/hashlib', encoding=[{'text':'中文𠀀','hex':'d6d0cec495328236'}], xml=vectors, digest=digest,
               rfc2617=dict(username='Mufasa', realm='testrealm@host.com', password='Circle Of Life', method='GET', uri='/dir/index.html', nonce='dcd98b7102dd2f0e8b11d0f600bfb0c093', nc='00000001', cnonce='0a4f113b', response='6629fae49393a05397450978507c4ef1'))
    assert '中文𠀀'.encode('gb18030').hex() == obj['encoding'][0]['hex']
    target = ROOT/'protocol-assets/m0-vectors.json'
    target.write_text(json.dumps(obj, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
    print('Wrote independent synthetic vectors:', len(vectors), 'XML + GB18030 + Digest')


if __name__ == '__main__':
    main()

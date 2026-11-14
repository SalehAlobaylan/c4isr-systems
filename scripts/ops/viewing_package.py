"""Issue, verify and serve a portable geographic viewing package on loopback."""
import argparse
from datetime import datetime, timezone
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import mimetypes
from pathlib import Path, PurePosixPath
import re
import shutil
import sys
from tempfile import TemporaryDirectory
from urllib.parse import urlsplit
import zipfile

VIEWER_VERSION = 'emad-geographic-viewer-v1'
MAX_ARCHIVE = 256 << 20
MAX_MEMBER = 32 << 20
DISPLAY_ID = re.compile(r'geodisplay_[a-f0-9]{64}\Z')
RELEASE_ID = re.compile(r'georelease_[a-z0-9]{26}\Z')
DIGEST = re.compile(r'[a-f0-9]{64}\Z')
RECIPES = {'terrain-glb-v1', 'terrain-glb-v2', 'imagery-display-v1', 'roads-display-v1', 'buildings-display-v1', 'water-display-v1', 'street-pmtiles-v1',
           'land-cover-display-v1', 'ground-delta-display-v1', 'ground-glb-v1', 'radar-display-v1'}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode('utf-8') + b'\n'


def safe_name(name):
    path = PurePosixPath(name)
    return (name == path.as_posix() and not path.is_absolute() and len(name) <= 240 and '..' not in path.parts
            and re.fullmatch(r'[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*', name) is not None)


def viewer_assets(root):
    assets = {}
    for file in sorted(root.rglob('*')):
        if file.is_symlink():
            raise ValueError(f'Viewer assets must be ordinary files: {file.name}')
        if file.is_file():
            name = file.relative_to(root).as_posix()
            if not safe_name(name) or name in ('viewer.json', 'navigation.json', 'issuance.json') or name.startswith('displays/'):
                raise ValueError(f'Unexpected viewer member: {name}')
            if file.stat().st_size > MAX_MEMBER:
                raise ValueError(f'Viewer member exceeds the byte budget: {name}')
            assets[name] = file.read_bytes()
    return assets


def verify(archive, expected, destination=None):
    if not DIGEST.fullmatch(expected):
        raise ValueError('Use the trusted SHA-256 supplied by the issuing API for issuance.json')
    items, total = {}, 0
    for entry in archive.infolist():
        name = entry.filename
        total += entry.file_size
        if (not safe_name(name) or name in items or entry.is_dir() or entry.external_attr >> 16 & 0o170000 == 0o120000
                or not 0 < entry.file_size <= MAX_MEMBER or total > MAX_ARCHIVE or len(items) >= 512):
            raise ValueError(f'Invalid or oversized viewing package member: {name}')
        items[name] = entry

    def checked(name, digest):
        if name not in items:
            raise ValueError(f'Missing viewing dependency: {name}')
        data = archive.read(items[name])
        if sha(data) != digest:
            raise ValueError(f'Viewing dependency SHA-256 differs: {name}')
        return data

    issuance = json.loads(checked('issuance.json', expected))
    if issuance['schema'] != 'emad.viewing-issuance.v1' or issuance['viewerVersion'] != VIEWER_VERSION:
        raise ValueError('Unsupported viewer or issuance version')
    expiry = datetime.fromisoformat(issuance['expiresAt'])
    if expiry.tzinfo is None or expiry <= datetime.now(timezone.utc):
        raise ValueError('The viewing authorization has expired. Apply the issued retention terms.')
    if not RELEASE_ID.fullmatch(issuance['releaseId']) or not DIGEST.fullmatch(issuance['manifestSha256']) or not issuance['terms']:
        raise ValueError('Issuance does not identify a release and its permitted use terms')
    navigation = json.loads(checked('navigation.json', issuance['navigationSha256']))
    if navigation['schema'] != 'emad.local-navigation.v1' or not navigation['features']['features'] or not navigation['sources']:
        raise ValueError('Local navigation dependencies are incomplete')
    viewer = json.loads(checked('viewer.json', issuance['viewerSha256']))
    dependencies = dict(viewer['files'])
    if (viewer['version'] != VIEWER_VERSION or 'index.html' not in dependencies
            or not all(any(name.endswith(ext) for name in dependencies) for ext in ('.js', '.css', '.woff2'))
            or not {'licenses/ibm-plex-sans.txt', 'licenses/ibm-plex-sans-arabic.txt', 'launcher/viewing_package.py'} <= dependencies.keys()):
        raise ValueError('The viewer is missing its launcher page, scripts, style, fonts or font licenses')
    recipes = set()
    lengths = {}
    for display in issuance['displays']:
        identifier, recipe = display['id'], display['recipe']
        if (not DISPLAY_ID.fullmatch(identifier) or recipe not in RECIPES or recipe in recipes
                or display['state'] != 'READY' or display['releaseId'] != issuance['releaseId']
                or display['manifestSha256'] != issuance['manifestSha256']):
            raise ValueError('A display differs from the issued release or recipe')
        name = f'displays/{identifier}'
        if name in dependencies:
            raise ValueError('Duplicate viewing dependency')
        recipes.add(recipe)
        dependencies[name] = display['sha256']
        lengths[name] = display['bytes']
    if not {'terrain-glb-v1', 'terrain-glb-v2'} & recipes:
        raise ValueError('The issued viewer requires terrain')
    roots = {'viewer.json': issuance['viewerSha256'], 'navigation.json': issuance['navigationSha256'], 'issuance.json': expected}
    if dependencies.keys() & roots.keys():
        raise ValueError('Viewer files collide with package metadata')
    dependencies.update(roots)
    if items.keys() - dependencies.keys():
        raise ValueError('The viewing package contains unissued members')
    for name, digest in dependencies.items():
        data = checked(name, digest)
        if name in lengths and len(data) != lengths[name]:
            raise ValueError(f'Issued byte length differs: {name}')
        if destination:
            path = destination / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
    return issuance, dependencies


def export(args):
    import requests

    sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'data'))
    from intake_environment import LibraryAPI

    if not RELEASE_ID.fullmatch(args.release) or not all(DISPLAY_ID.fullmatch(identifier) for identifier in args.display):
        raise ValueError('Supply exact release and display IDs')
    if args.package.exists():
        raise FileExistsError('Viewing package already exists')
    assets = viewer_assets(args.viewer)
    viewer = encoded({'version': VIEWER_VERSION, 'files': {name: sha(raw) for name, raw in assets.items()}})
    if args.navigation.stat().st_size > MAX_MEMBER:
        raise ValueError('Local navigation exceeds the package budget')
    nav = args.navigation.read_bytes()
    api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
    request = {'manifestSha256': args.manifest_sha256, 'viewerSha256': sha(viewer), 'navigationSha256': sha(nav), 'displays': args.display}
    if args.expires_at:
        request['expiresAt'] = args.expires_at
    response = requests.post(f'{api.origin}/environment-releases/{args.release}/viewing-packages', json=request,
                             headers=api.headers, timeout=30, allow_redirects=False)
    if response.status_code != 201:
        raise ValueError(f'Viewing issuance was denied (HTTP {response.status_code}). {response.json().get("error", {}).get("message", "")}'.strip())
    receipt = response.content
    issuance = response.json()
    assets.update({'issuance.json': receipt, 'navigation.json': nav, 'viewer.json': viewer})
    for display in issuance['displays']:
        with requests.get(f'{api.origin}/environment-displays/{display["id"]}/content', headers=api.headers,
                          stream=True, timeout=(10, 90), allow_redirects=False) as response:
            if (response.status_code != 200 or response.headers.get('ETag') != f'"{display["sha256"]}"'
                    or int(response.headers.get('Content-Length', '0')) != display['bytes'] or not 0 < display['bytes'] <= MAX_MEMBER):
                raise ValueError(f'Issued display is no longer available: {display["id"]}')
            content = bytearray()
            for data in response.iter_content(256 << 10):
                content.extend(data)
                if len(content) > display['bytes']:
                    raise ValueError('Issued display exceeds its declared size')
            assets[f'displays/{display["id"]}'] = bytes(content)
    with TemporaryDirectory(prefix='emad-viewing-build-') as directory:
        temporary = Path(directory) / 'viewing.zip'
        with zipfile.ZipFile(temporary, 'x', zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
            for name, raw in sorted(assets.items()):
                archive.writestr(name, raw)
        with zipfile.ZipFile(temporary) as archive:
            verify(archive, sha(receipt))
        with temporary.open('rb') as src, args.package.open('xb') as dst:
            shutil.copyfileobj(src, dst, 256 << 10)
    print(json.dumps({'issuanceSha256': sha(receipt), 'issuanceId': issuance['id'], 'expiresAt': issuance['expiresAt']}))


def serve(directory, dependencies, expiry, port):
    class ViewerHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.headers.get('Host') != f'127.0.0.1:{self.server.server_port}':
                self.send_error(403)
                return
            if expiry <= datetime.now(timezone.utc):
                self.send_error(410, 'The viewing authorization has expired')
                return
            name = urlsplit(self.path).path.removeprefix('/') or 'index.html'
            if name not in dependencies:
                self.send_error(404)
                return
            path = directory / name
            self.send_response(200)
            mime = {'.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json'}.get(path.suffix)
            self.send_header('Content-Type', mime or mimetypes.guess_type(name)[0] or 'application/octet-stream')
            self.send_header('Content-Length', str(path.stat().st_size))
            self.send_header('ETag', f'"{dependencies[name]}"')
            self.send_header('Cache-Control', 'no-store')
            self.send_header('X-Content-Type-Options', 'nosniff')
            self.send_header('Cross-Origin-Resource-Policy', 'same-origin')
            self.send_header('Content-Security-Policy', "default-src 'none'; script-src 'self'; connect-src 'self'; "
                             "worker-src 'self' blob:; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' blob: data:; frame-ancestors 'none'")
            self.end_headers()
            with path.open('rb') as stream:
                shutil.copyfileobj(stream, self.wfile, 256 << 10)

    with ThreadingHTTPServer(('127.0.0.1', port), ViewerHandler) as server:
        print(f'http://127.0.0.1:{server.server_port}/', flush=True)
        server.serve_forever()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    for command in ('verify', 'serve'):
        operation = commands.add_parser(command)
        operation.add_argument('package', type=Path)
        operation.add_argument('--issuance-sha256', required=True)
        if command == 'serve':
            operation.add_argument('--port', type=int, default=0)
    operation = commands.add_parser('export')
    operation.add_argument('package', type=Path)
    for field in ('api', 'workspace', 'environment', 'release', 'manifest-sha256'):
        operation.add_argument('--' + field, required=True)
    for field in ('token-file', 'viewer', 'navigation'):
        operation.add_argument('--' + field, type=Path, required=True)
    operation.add_argument('--display', action='append', required=True)
    operation.add_argument('--expires-at')
    args = parser.parse_args()
    try:
        if args.command == 'export':
            export(args)
        elif args.command == 'verify':
            with zipfile.ZipFile(args.package) as archive:
                issuance, dependencies = verify(archive, args.issuance_sha256)
            print(json.dumps({'status': 'PASS', 'release': issuance['releaseId'], 'expiresAt': issuance['expiresAt'], 'files': len(dependencies)}))
        else:
            with TemporaryDirectory(prefix='emad-geographic-viewer-') as folder:
                with zipfile.ZipFile(args.package) as archive:
                    issuance, dependencies = verify(archive, args.issuance_sha256, Path(folder))
                serve(Path(folder), dependencies, datetime.fromisoformat(issuance['expiresAt']), args.port)
    except KeyboardInterrupt:
        return
    except (KeyError, TypeError, ValueError, OSError, zipfile.BadZipFile) as error:
        parser.exit(2, f'{error}\n')


if __name__ == '__main__':
    main()

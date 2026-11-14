import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import socket
from tempfile import TemporaryDirectory
from threading import Thread
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from check_installed_environment_library import download


class ResumedDownloadTest(unittest.TestCase):
    def test_interruption_resumes_only_exact_representation(self):
        payload = b"geographic-fixture" * 45000
        digest = hashlib.sha256(payload).hexdigest()
        tag = f'"{digest}"'
        cut = 2 * (256 << 10)

        for wrong_identity in (False, True):
            with self.subTest(wrong_identity=wrong_identity), TemporaryDirectory() as directory:
                seen = []

                class Server(BaseHTTPRequestHandler):
                    def log_message(self, *_):
                        pass

                    def do_GET(self):
                        seen.append((self.headers.get("Range"), self.headers.get("If-Range")))
                        if len(seen) == 1:
                            self.send_response(200)
                            self.send_header("ETag", tag)
                            self.send_header("Content-Length", str(len(payload)))
                            self.end_headers()
                            self.wfile.write(payload[:cut])
                            self.connection.shutdown(socket.SHUT_WR)
                        else:
                            self.send_response(206)
                            self.send_header("ETag", '"' + "0" * 64 + '"' if wrong_identity else tag)
                            self.send_header("Content-Length", str(len(payload) - cut))
                            self.send_header("Content-Range", f"bytes {cut}-{len(payload) - 1}/{len(payload)}")
                            self.end_headers()
                            self.wfile.write(payload[cut:])

                with ThreadingHTTPServer(("127.0.0.1", 0), Server) as server:
                    worker = Thread(target=server.serve_forever)
                    worker.start()
                    try:
                        api = SimpleNamespace(origin=f"http://127.0.0.1:{server.server_port}", headers={})
                        target = Path(directory) / "prepared.zip"
                        with patch("check_installed_environment_library.verify_prepared", return_value={}) as verify:
                            if wrong_identity:
                                with self.assertRaisesRegex(RuntimeError, "identity or encoding changed"):
                                    download(api, "fixture", target, "manifest", digest)
                                self.assertEqual(list(target.parent.iterdir()), [])
                                verify.assert_not_called()
                            else:
                                proof = download(api, "fixture", target, "manifest", digest)
                                self.assertEqual(target.read_bytes(), payload)
                                self.assertEqual(proof["preparedBytes"], len(payload))
                                verify.assert_called_once()
                        self.assertEqual(seen, [(None, None), (f"bytes={cut}-", tag)])
                    finally:
                        server.shutdown()
                        worker.join()

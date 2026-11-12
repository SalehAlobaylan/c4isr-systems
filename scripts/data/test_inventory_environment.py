import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from inventory_environment import contained, inventory, public_location


class EnvironmentInventoryTests(unittest.TestCase):
    def fixture(self, directory):
        root = Path(directory)
        (root / 'raw').mkdir()
        data = b'retained source bytes'
        (root / 'raw' / 'terrain.bin').write_bytes(data)
        receipt = {'sha256': hashlib.sha256(data).hexdigest(), 'bytes': len(data),
                   'url': 'https://user:secret@example.invalid/terrain?token=private#fragment',
                   'retrievedAt': '2026-01-01T00:00:00Z'}
        (root / 'raw' / 'terrain.bin.receipt.json').write_text(json.dumps(receipt), encoding='utf-8')
        return root

    def test_integrity_inventory_does_not_promote_or_expose_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            root = self.fixture(directory)
            report = inventory(root)
            self.assertEqual(report['summary']['verifiedAssets'], 1)
            self.assertEqual(report['status'], 'INVENTORIED_NOT_QUALIFIED')
            self.assertEqual(report['assets'][0]['sourceLocation'], 'https://example.invalid/terrain')
            self.assertNotIn('secret', json.dumps(report))
            self.assertNotIn('token=', json.dumps(report))

    def test_changed_bytes_are_reported_even_when_size_matches(self):
        with tempfile.TemporaryDirectory() as directory:
            root = self.fixture(directory)
            path = root / 'raw' / 'terrain.bin'
            path.write_bytes(b'x' * path.stat().st_size)
            report = inventory(root)
            self.assertEqual(report['assets'][0]['integrity'], 'FAIL')
            self.assertEqual(report['summary']['verifiedAssets'], 0)

    def test_missing_parent_and_empty_collection_cannot_pass(self):
        with tempfile.TemporaryDirectory() as directory:
            root = self.fixture(directory)
            (root / 'raw' / 'terrain.bin').unlink()
            self.assertEqual(inventory(root)['assets'][0]['integrity'], 'UNVERIFIED')
            (root / 'raw' / 'terrain.bin.receipt.json').unlink()
            self.assertIn({'code': 'NO_RECEIPTED_RAW_INPUTS'}, inventory(root)['issues'])

    def test_duplicate_content_preserves_both_receipts(self):
        with tempfile.TemporaryDirectory() as directory:
            root = self.fixture(directory)
            (root / 'raw' / 'copy.bin').write_bytes((root / 'raw' / 'terrain.bin').read_bytes())
            (root / 'raw' / 'copy.bin.receipt.json').write_bytes((root / 'raw' / 'terrain.bin.receipt.json').read_bytes())
            report = inventory(root)
            self.assertEqual(report['summary']['receiptedAssets'], 2)
            self.assertEqual(report['summary']['uniqueContentHashes'], 1)
            self.assertEqual(len(report['duplicateByteGroups']), 1)

    def test_root_escape_and_non_web_locations_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            with self.assertRaises(ValueError):
                contained(root, root / '..' / 'outside')
        self.assertIsNone(public_location('file:///private/token'))


if __name__ == '__main__':
    unittest.main()

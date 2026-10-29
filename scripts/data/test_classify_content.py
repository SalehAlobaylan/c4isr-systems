import copy
import unittest

from classify_content import run


class ReceiptAPI:
    def __init__(self, purpose='unclassified', version=0):
        self.original = dict(id='exact-reference', sha256='retained-hash', version='v1', datasetKey='source-backed-key')
        self.metadata = dict(kind='reference', entityId='exact-reference', purpose=purpose, version=version)
        self.writes = []

    def call(self, method, path, body=None):
        if path == '/reference-datasets/exact-reference':
            return copy.deepcopy(self.original)
        if path.startswith('/content-purpose?'):
            rows = [self.metadata] if 'kind=reference' in path else []
            return dict(items=copy.deepcopy(rows), hasMore=False)
        if method == 'POST' and path == '/content-purpose':
            self.writes.append(copy.deepcopy(body))
            return dict(body, version=body['expectedVersion'] + 1)
        raise AssertionError((method, path))


class ClassificationReceiptTests(unittest.TestCase):
    def receipts(self):
        return {'references': [dict(id='exact-reference', sha256='retained-hash', version='v1', datasetKey='source-backed-key')]}

    def test_preview_does_not_write_and_keeps_unavailable_ids_explicit(self):
        api = ReceiptAPI()
        report = run(api, self.receipts())
        self.assertFalse(report['applied'])
        self.assertFalse(api.writes)
        self.assertEqual(report['entries'][0]['entityId'], 'exact-reference')
        self.assertEqual(report['entries'][0]['state'], 'PLANNED')
        self.assertTrue(any(row['state'] == 'NOT_PRESENT_OR_NOT_AUTHORIZED' for row in report['entries']))

    def test_apply_uses_verified_id_and_never_rewrites_original(self):
        api = ReceiptAPI()
        original = copy.deepcopy(api.original)
        run(api, self.receipts(), apply=True)
        self.assertEqual(len(api.writes), 1)
        self.assertEqual(api.writes[0]['entityId'], 'exact-reference')
        self.assertEqual(api.writes[0]['expectedVersion'], 0)
        self.assertEqual(api.original, original)

    def test_hash_mismatch_stops_before_classification(self):
        api = ReceiptAPI()
        receipts = self.receipts()
        receipts['references'][0]['sha256'] = 'different-hash'
        with self.assertRaisesRegex(RuntimeError, 'does not match'):
            run(api, receipts, apply=True)
        self.assertFalse(api.writes)

    def test_existing_human_classification_is_not_overwritten(self):
        api = ReceiptAPI(purpose='development', version=2)
        with self.assertRaisesRegex(RuntimeError, 'conflicts'):
            run(api, self.receipts(), apply=True)
        self.assertFalse(api.writes)


if __name__ == '__main__':
    unittest.main()

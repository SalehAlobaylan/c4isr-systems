import copy
import tempfile
import unittest
from pathlib import Path
from shapely.geometry import box, Point
from regional_geography import prepare, install, bounded_chunks, LAYERS


class RegionalGeographyTests(unittest.TestCase):
    def setUp(self):
        self.profile={'schema':'emad.region-install-profile.v1','key':'portable-fixture','name':'Portable fixture','definition':'Explicit synthetic source-backed test selection','countries':['AAA'],'studyAreas':[]}
        self.rows={LAYERS[0][0]:[({'ADM0_A3':'AAA','NAME_EN':'Fixture country'},box(1,1,3,3))],
                   LAYERS[1][0]:[({'adm0_a3':'AAA','name':'Source district','ne_id':41},box(1,1,3,3))],
                   LAYERS[2][0]:[({'ADM0_A3':'AAA','NAME':'Source place','NAME_AR':'موضع','NE_ID':701},Point(2,2)),({'ADM0_A3':'AAA','NAME':'Second place','NE_ID':700},Point(2.5,2))]}
    def loader(self,dataset,name):
        return self.rows[dataset],{'sha256':'retained-source-fixture','bytes':1,'url':'https://naturalearth.s3.amazonaws.com/fixture.zip'}
    def test_stable_source_ids_and_manifest_across_row_order(self):
        first=prepare(self.profile,self.loader)
        for rows in self.rows.values():rows.reverse()
        second=prepare(self.profile,self.loader)
        self.assertEqual(first['version'],second['version'])
        self.assertEqual(first['chunks'],second['chunks'])
        self.assertEqual(first['depth']['AAA'],{'regions':1,'places':2})
    def test_missing_source_identity_is_explicit_not_row_offset(self):
        self.rows[LAYERS[2][0]][0][0].pop('NE_ID')
        with self.assertRaisesRegex(ValueError,'stable source NE_ID'):prepare(self.profile,self.loader)
    def test_unavailable_country_never_borrows_installed_geometry(self):
        self.profile['countries']=['BBB']
        with self.assertRaisesRegex(ValueError,'unavailable'):prepare(self.profile,self.loader)
    def test_changed_retained_receipt_gets_a_distinct_release_version(self):
        first=prepare(self.profile,self.loader)
        def changed(dataset,name):
            rows,receipt=self.loader(dataset,name)
            return rows,{**receipt,'sha256':'another-retained-upstream-edition'}
        second=prepare(self.profile,changed)
        self.assertNotEqual(first['version'],second['version'])
    def test_chunks_stay_bounded_and_do_not_simplify(self):
        import base64,json
        plan=prepare(self.profile,self.loader)
        features=json.loads(base64.b64decode(plan['chunks'][2]['artifact']))['features']
        chunks=list(bounded_chunks(features,max_features=1))
        self.assertEqual([len(c) for c in chunks],[1,1])
        self.assertIn('موضع',str(chunks))
        self.assertEqual(chunks[1][0]['geometry']['coordinates'],[2.0,2.0])
    def test_resume_never_activates_a_partial_release(self):
        class API:
            def __init__(self):self.editions={};self.fail=True;self.activated=False
            def call(self,method,path,value=None):
                if method=='GET':return {'items':[]}
                if path=='/geography/editions':
                    if self.fail and len(self.editions)==1:raise RuntimeError('synthetic interrupted import')
                    assert value['staged'] is True
                    key=value['sha256'];self.editions[key]={'id':'edition-'+key};return self.editions[key]
                if path=='/geography/releases':
                    assert len(value['editionIds'])==3
                    self.activated=True;return {'id':'release-fixture'}
                if path=='/geography/profiles':return {'id':'profile-fixture'}
                raise AssertionError(path)
        api=API()
        with tempfile.TemporaryDirectory() as directory:
            output=Path(directory)/'progress.json'
            with self.assertRaisesRegex(RuntimeError,'interrupted'):install(api,self.profile,self.loader,output)
            self.assertFalse(api.activated)
            api.fail=False
            installed=install(api,self.profile,self.loader,output)
            self.assertTrue(api.activated)
            self.assertEqual(len(installed),3)
            self.assertIn('"state":"ACTIVE"',output.read_text(encoding='utf-8'))

if __name__=='__main__':unittest.main()

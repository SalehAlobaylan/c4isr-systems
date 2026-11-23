from datetime import datetime, timezone
import json
from pathlib import Path
import tempfile
import unittest

from evaluate_supporting_sources import dust, navigation, passenger_count
from prepare_supporting_sources import cap, regional_column, reviewed_indicators


class SupportingSourcesTests(unittest.TestCase):
    def test_cap_preserves_translations_original_ids_and_revision_cutoffs(self):
        def notice(identity, sent, kind, references="", info=""):
            return f'''<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>{identity}</identifier>
<sender>authored@example.invalid</sender><sent>{sent}</sent><msgType>{kind}</msgType><references>{references}</references>{info}</alert>'''
        with tempfile.TemporaryDirectory() as d:
            create, update, cancel = [Path(d) / name for name in ("create.xml", "update.xml", "cancel.xml")]
            a,b,c = [f"2024-01-01T0{hour}:00:00Z" for hour in (1,2,3)]
            create.write_text(notice("one", a, "Alert", info="<info><language>ar-SA</language><headline>تنبيه</headline></info><info><language>en</language><headline>Alert</headline></info>"), encoding="utf-8")
            update.write_text(notice("two", b, "Update", f"authored@example.invalid,one,{a}"), encoding="utf-8")
            cancel.write_text(notice("three", c, "Cancel", f"authored@example.invalid,two,{b}"), encoding="utf-8")
            captures = [datetime(2024,1,1,hour,5,tzinfo=timezone.utc) for hour in (1,2,3)]
            result = cap([create,update,cancel], captures, "Training area")
            self.assertEqual(len({e["origin"] for e in result["events"]}),1)
            self.assertEqual(result["events"][0]["languages"]["ar-SA"],"تنبيه")
            self.assertEqual([e["revision"] for e in result["events"]],[1,2,3])
            self.assertEqual(result["events"][2]["status"],"cancelled")
            self.assertEqual(result["events"][1]["availableAt"],"2024-01-01T02:05:00Z")
            with self.assertRaisesRegex(ValueError, "issuer references"):
                cap([update,create], captures[:2], "Training area")

    def test_column_and_indicator_capture_distinguish_missing_and_valid_zero(self):
        with tempfile.TemporaryDirectory() as d:
            pixels, scope, extract = [Path(d) / name for name in ("pixels.json", "scope.json", "capture.json")]
            pixels.write_text(json.dumps({"reviewAreaM2":100, "minQuality":.75, "pixels":[
                {"id":"valid","areaM2":20,"quality":.9,"value":0},{"id":"cloud","areaM2":75,"quality":.2,"value":3,"reason":"cloud"}
            ]}))
            result = regional_column(pixels,{})
            self.assertEqual((result["coverage"],result["samples"],result["value"]),(.2,1,0))
            scope.write_text(json.dumps({"provider":"hdx-hapi","edition":"test","sourceId":"owned","scopes":[{"id":"observed"},{"id":"missing"}]}))
            extract.write_text(json.dumps({"status":"complete","rows":[{"scopeId":"observed","value":0}]}))
            values = reviewed_indicators(extract,scope)["indicators"]
            self.assertEqual([(i["status"],i["value"]) for i in values], [("available",0),("no-coverage",None)])
            extract.write_text(json.dumps({"status":"source-error","rows":[]}))
            self.assertEqual(reviewed_indicators(extract,scope)["indicators"][0]["status"],"source-error")

    def test_evaluators_preserve_uncertainty_and_denominators(self):
        ref = {"cases":[{"id":"control","state":"missing"}]}
        submission = {"cases":[{"id":"control","state":"degraded","cause":"asserted","requestObservation":True}]}
        self.assertEqual(navigation(ref,submission)["cases"][0],{"id":"control","coverageStateCorrect":False,
                                                               "preservedCauseUncertainty":False,"soughtObservation":True})
        quality = {"cases":[{"id":"first","group":"isolated","split":"test","imageQuality":"unusable"}]}
        self.assertEqual(dust(quality,{"cases":[{"id":"first","prediction":"usable"}]})["falseUsable"],1)
        quality["cases"].append({"id":"second","group":"isolated","split":"development"})
        with self.assertRaisesRegex(ValueError,"blind split"):
            dust(quality,{"cases":[]})
        count = {"airport":"owned","day":"2024-01-01","cancellationDefinition":"full day","scheduled":0,"cancelled":0}
        self.assertIsNone(passenger_count(count,count)["cancelledFraction"])
        with self.assertRaisesRegex(ValueError,"exceed"):
            passenger_count({**count,"cancelled":1},{**count,"cancelled":1})


if __name__ == "__main__":
    unittest.main()

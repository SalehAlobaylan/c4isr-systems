import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest

import observation_formats as f


class ObservationTests(unittest.TestCase):
    def test_field_contract_rejects_invented_resolution_and_below_ground_values(self):
        slot = 559356
        keys = [".zmetadata","latitude/0","longitude/0","level/0","time/7",
                f"surface_pressure/{slot}.0.0",*(f"{key}/{slot}.0.0.0" for key in f.WEATHER_VARS)]
        sample = {"schema":"emad.era5-pressure-subset.v1","edition":f.WEATHER_EDITION,"crs":"EPSG:4326",
                  "verticalCoordinate":"pressure-hPa","physicalAccuracy":"UNKNOWN","nativeSpacingDegrees":.25,
                  "validTimes":["2022-10-23T12:00:00Z"],"pressureHPa":[850],"latitudes":[25.,24.75],"longitudes":[46.5,46.75],
                  "aboveGroundMask":[True,True,False,True],
                  "variables":{k:{"units":v[0],"samples":[290 if k=="temperature" else 0, None, None, None]} for k,v in f.WEATHER_VARS.items()},
                  "sourceObjects":[{"key":key,"sha256":"0"*64,"bytes":1} for key in keys]}
        self.assertEqual(f.validate_weather(sample),4)
        corrupt=copy.deepcopy(sample)
        corrupt["variables"]["temperature"]["samples"][2]=295
        with self.assertRaisesRegex(ValueError,"below-ground"):
            f.validate_weather(corrupt)
        corrupt=copy.deepcopy(sample)
        corrupt["longitudes"][1]=46.625
        with self.assertRaisesRegex(ValueError,"coordinate"):
            f.validate_weather(corrupt)
        corrupt=copy.deepcopy(sample)
        corrupt["sourceObjects"][-1]["key"]="unrelated"
        with self.assertRaisesRegex(ValueError,"receipts"):
            f.validate_weather(corrupt)

    def test_captured_radar_uses_the_published_scene_and_rejects_other_mode(self):
        root=os.getenv("EMAD_OBSERVATION_PILOT")
        if not root:
            self.skipTest("Set EMAD_OBSERVATION_PILOT to the retained provider sample directory")
        with tempfile.TemporaryDirectory() as directory:
            raster=Path(directory)/"native.tif"
            item,meta,receipt=f.radar_archive(Path(root)/"rtc/radar-capture.zip",raster)
            self.assertEqual(hashlib.sha256(raster.read_bytes()).hexdigest(),receipt["retainedSha256"])
            self.assertEqual(meta["polarization"],"VV")
            item["properties"]["sar:instrument_mode"]="EW"
            with self.assertRaisesRegex(ValueError,"mode"):
                f.radar_metadata(item)
            weather=json.loads((Path(root)/"era/era5-pressure-subset.json").read_bytes())
            self.assertEqual(f.validate_weather(weather),98)
            self.assertEqual(weather["variables"]["temperature"]["samples"][0],296.0613098144531)


if __name__=="__main__":
    unittest.main()

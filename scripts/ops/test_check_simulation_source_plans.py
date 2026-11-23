"""Regression cases for plan coverage and the two historical port-day controls."""
import json
import unittest

import check_simulation_source_plans as harness


def port_sample(*rows, **metadata):
    return json.dumps({"features": [{"attributes": row} for row in rows], **metadata}).encode()


class InputMappingTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.document = harness.read(harness.DIRECTORY / "normalization.md")
        cls.adapters = harness.registered_adapters()

    def test_swapped_registered_adapter_is_rejected(self):
        changed = self.document.replace(self.adapters["usgs"], self.adapters["aisstream"])
        with self.assertRaisesRegex(ValueError, "usgs: expected registered adapter"):
            harness.dictionary(changed, self.adapters)

    def test_unverified_pilot_promotion_is_rejected(self):
        changed = self.document.replace("design/aggregate", "pilot/grid", 1)
        with self.assertRaisesRegex(ValueError, "unknown integration path"):
            harness.dictionary(changed, self.adapters)

    def test_source_section_requires_a_mapping(self):
        with self.assertRaisesRegex(ValueError, "needs one field mapping"):
            harness.mapped_inputs(harness.PLANS[0], "### New municipal input\n\nData to be collected.")


class PortDayTests(unittest.TestCase):
    def setUp(self):
        self.jubail = dict(year=2024, month=1, day=15, portid="port24", portname="Jubail", portcalls=4)
        self.jazan = dict(self.jubail, portid="port2074", portname="Jazan", portcalls=0)

    def test_measured_zero_is_accepted(self):
        harness.check_ports(port_sample(self.jubail, self.jazan))

    def test_gaps_are_distinct_from_zero(self):
        with self.assertRaisesRegex(ValueError, "missing or changed control row"):
            harness.check_ports(port_sample(self.jubail))
        with self.assertRaisesRegex(ValueError, "invalid port call count"):
            harness.check_ports(port_sample(self.jubail, dict(self.jazan, portcalls=None)))

    def test_wrong_date_duplicate_and_partial_results_fail(self):
        cases = (
            (port_sample(dict(self.jubail, day=16), self.jazan), "unexpected port-day"),
            (port_sample(self.jubail, self.jazan, self.jazan), "duplicate port-day"),
            (port_sample(self.jubail, self.jazan, exceededTransferLimit=True), "failed or partial"),
        )
        for sample, reason in cases:
            with self.subTest(reason=reason), self.assertRaisesRegex(ValueError, reason):
                harness.check_ports(sample)

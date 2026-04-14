import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("generate_reference.py")
PDF_PATH = Path(__file__).with_name("ST0601.19.pdf")


def load_module():
    spec = importlib.util.spec_from_file_location("generate_reference", SCRIPT_PATH)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class GenerateReferenceTests(unittest.TestCase):
    def test_parse_summary_block_returns_atomic_fields(self):
        mod = load_module()
        summary = """8.103            Item 103: Density Altitude Extended
                                                   Description
 Density altitude above MSL at aircraft location
         Units                                      Format                  Min            Max             Offset
         Meters              Software               float64                -900            40000
           (m)                  KLV                  IMAPB                  N/A            N/A              N/A
               Length                                   Max Length                           Required Length
              Variable                                         8                                     N/A

            Resolution                                                    Special Values
                 N/A                                                           None

Required in LS?          Optional      Allowed in SDCC Pack?                  No      Multiples Allowed?          No

 Software Value To KLV Value                        KLVval = IMAPB(−900, 40000, Length, Soft Val )
 KLV Value To Software Value                       Soft val = RIMAPB(−900, 40000, Length, KLVuint )
              Example Software Value                                  Example KLV Item (All Hex)
                                                              Tag   Len                      Value
                   23,456.24 Meters
                                                              67    03                     2F92 1E

• Relative aircraft performance metric based on outside air temperature, static pressure, and humidity
• Max Altitude: 40,000m for airborne systems
• For resolution < 1.0m, a length of >= 3 bytes is required
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_description"], "Density altitude above MSL at aircraft location")
        self.assertEqual(fields["pdf_units"], "Meters (m)")
        self.assertEqual(fields["pdf_software_format"], "float64")
        self.assertEqual(fields["pdf_software_min"], "-900")
        self.assertEqual(fields["pdf_software_max"], "40000")
        self.assertEqual(fields["pdf_klv_format"], "IMAPB")
        self.assertEqual(fields["pdf_klv_min"], "N/A")
        self.assertEqual(fields["pdf_klv_max"], "N/A")
        self.assertEqual(fields["pdf_offset"], "N/A")
        self.assertEqual(fields["pdf_length"], "Variable")
        self.assertEqual(fields["pdf_max_length"], "8")
        self.assertEqual(fields["pdf_required_length"], "N/A")
        self.assertEqual(fields["pdf_resolution"], "N/A")
        self.assertEqual(fields["pdf_special_values"], "None")
        self.assertEqual(fields["pdf_required_in_ls"], "Optional")
        self.assertEqual(fields["pdf_allowed_in_sdcc_pack"], "No")
        self.assertEqual(fields["pdf_multiples_allowed"], "No")
        self.assertEqual(fields["pdf_software_value_to_klv_value"], "KLVval = IMAPB(−900, 40000, Length, Soft Val )")
        self.assertEqual(fields["pdf_klv_value_to_software_value"], "Soft val = RIMAPB(−900, 40000, Length, KLVuint )")
        self.assertEqual(fields["pdf_example_software_value"], "23,456.24 Meters")
        self.assertEqual(fields["pdf_example_klv_item"], "Tag=67 Len=03 Value=2F92 1E")
        self.assertEqual(
            fields["pdf_notes"],
            [
                "Relative aircraft performance metric based on outside air temperature, static pressure, and humidity",
                "Max Altitude: 40,000m for airborne systems",
                "For resolution < 1.0m, a length of >= 3 bytes is required",
            ],
        )

    def test_clean_details_strips_layout_noise(self):
        mod = load_module()
        details = """The Metadata Substream ID (MSID) Pack identifies a particular Segment LS.

02 March 2023                             Motion Imagery Standards Board                                          224
Figure 90: MSID Pack
ST 0601.19 UAS Datalink Local Set
ST 0601.19-46     Where a Metadata Substream ID (MSID) Pack includes a local identifier.
"""

        cleaned = mod.clean_details(details)

        self.assertIn("The Metadata Substream ID (MSID) Pack identifies a particular Segment LS.", cleaned)
        self.assertIn("ST 0601.19-46", cleaned)
        self.assertNotIn("Motion Imagery Standards Board", cleaned)
        self.assertNotIn("02 March 2023", cleaned)
        self.assertNotIn("Figure 90", cleaned)
        self.assertNotIn("ST 0601.19 UAS Datalink Local Set", cleaned)

    def test_parse_summary_block_handles_split_resolution_and_wrapped_formula(self):
        mod = load_module()
        summary = """8.23 Item 23: Frame Center Latitude
Description
Terrain latitude of frame center
Units Format Min Max Offset
Degrees Software float64 -90 90
(°) KLV int32 -((2^31)-1) (2^31)-1 None
Length Max Length Required Length
4 4 4

Resolution Special Values
~42 nanodegrees                          0x80000000 = "N/A (Off-Earth)" indicator

Required in LS? Optional Allowed in SDCC Pack? No Multiples Allowed? No

Software Value To KLV Value KLVval = ( ) ∗ Soft Val
LSrange 180
KLV Value To Software Value Soft val = ( ) ∗ KLVint = ( ) ∗ KLVint
int range 4294967294
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_resolution"], "~42 nanodegrees")
        self.assertEqual(fields["pdf_special_values"], '0x80000000 = "N/A (Off-Earth)" indicator')
        self.assertEqual(fields["pdf_software_value_to_klv_value"], "KLVval = (4294967294/180) * Soft Val")
        self.assertEqual(fields["pdf_klv_value_to_software_value"], "Soft val = (180/4294967294) * KLVint")

    def test_parse_summary_block_handles_na_example_row(self):
        mod = load_module()
        summary = """8.73 Item 73: RVT Local Set
Description
MISB ST 0806 RVT Local Set metadata items
Units Format Min Max Offset
None Software record N/A N/A
KLV set N/A N/A N/A
Length Max Length Required Length
Variable Not Limited N/A

Resolution Special Values
N/A N/A

Required in LS? Optional Allowed in SDCC Pack? No Multiples Allowed? No

Software Value To KLV Value See MISB ST 0806
KLV Value To Software Value See MISB ST 0806
Example Software Value Example KLV Item (All Hex)
Tag Len Value
N/A
49 - N/A
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_example_software_value"], "N/A")
        self.assertEqual(fields["pdf_example_klv_item"], "Tag=49 Len=- Value=N/A")

    def test_parse_summary_block_reconstructs_fraction_formula(self):
        mod = load_module()
        summary = """8.53 Item 53: Airfield Barometric Pressure
Description
Local pressure at airfield of known height
Units Format Min Max Offset
Millibar Software float32 0 5000
(mbar) KLV uint16 0 (2^16)-1 None
Length Max Length Required Length
2 2 2

Resolution Special Values
~0.08 millibar None

Required in LS? Optional Allowed in SDCC Pack? No Multiples Allowed? No

                                                                         65535
Software Value To KLV Value                                KLVval = (          ) ∗ Soft Val
                                                                          5000
                                                           LSrange                    5000
KLV Value To Software Value                  Soft val = (            ) ∗ KLVuint = (        ) ∗ KLVuint
                                                          uint range                 65535
             Example Software Value                                    Example KLV Item (All Hex)
                                                          Tag   Len                       Value
                2088.96010 Millibar
                                                          35    02                        6AF4
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_software_value_to_klv_value"], "KLVval = (65535/5000) * Soft Val")
        self.assertEqual(fields["pdf_klv_value_to_software_value"], "Soft val = (5000/65535) * KLVuint")

    def test_parse_summary_block_handles_imapb_resolution_none(self):
        mod = load_module()
        summary = """8.104            Item 104: Sensor Ellipsoid Height Extended
                                                 Description
 Sensor ellipsoid height extended as measured from the reference WGS84 ellipsoid
         Units                                   Format            Min                     Max             Offset
        Meters                Software           float64           -900                    40000
           (m)                  KLV               IMAPB             N/A                    N/A              N/A
               Length                                Max Length                              Required Length
             Variable                                          8                                     N/A

            Resolution                                                    Special Values
       2 bytes = 2 meters
                                                                               None
       3 bytes = 78.125 mm
Required in LS?         Optional      Allowed in SDCC Pack?                   Yes     Multiples Allowed?          No
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_resolution"], "2 bytes = 2 meters ; 3 bytes = 78.125 mm")
        self.assertEqual(fields["pdf_special_values"], "None")

    def test_parse_summary_block_handles_split_pack_example(self):
        mod = load_module()
        summary = """8.143            Item 143: Metadata Substream ID Pack
Description
Identifier for Segment or Amend items
Units Format Min Max Offset
None Software record N/A N/A
KLV dlp N/A N/A N/A
Length Max Length Required Length
Variable 17 N/A

Resolution Special Values
N/A N/A

Required in LS? Optional Allowed in SDCC Pack? No Multiples Allowed? No

Software Value To KLV Value KLVval = Soft val
KLV Value To Software Value Soft val = KLVval
Example Software Value Example KLV Item (All Hex)
Tag Len Value
00:8dc4f462-3ea2-5a85-9c5d-0af0c95e8c39                           008D C4F4 623E A25A 859C 5D0A F0C9 5E8C
                                                            810F   11
                                                                                        39
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_example_software_value"], "00:8dc4f462-3ea2-5a85-9c5d-0af0c95e8c39")
        self.assertEqual(fields["pdf_example_klv_item"], "Tag=810F Len=11 Value=008D C4F4 623E A25A 859C 5D0A F0C9 5E8C 39")

    def test_parse_summary_block_keeps_resolution_units_attached(self):
        mod = load_module()
        summary = """8.117           Item 117: Sensor Azimuth Rate
Description
The rate the sensors azimuth angle is changing
Units Format Min Max Offset
Degrees Per Second Software float32 -1000.0 1000.0
(dps) KLV IMAPB N/A N/A N/A
Length Max Length Required Length
Variable 4 N/A

Resolution Special Values
2 bytes = 0.0625
degrees/second
None
3 bytes = 0.000244
degrees/second
Required in LS? Optional Allowed in SDCC Pack? Yes Multiples Allowed? No
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(
            fields["pdf_resolution"],
            "2 bytes = 0.0625 degrees/second ; 3 bytes = 0.000244 degrees/second",
        )
        self.assertEqual(fields["pdf_special_values"], "None")

    def test_parse_summary_block_strips_header_and_bullet_from_complex_example(self):
        mod = load_module()
        summary = """8.138           Item 138: Payload List
Description
List of payloads available on the Platform
Units Format Min Max Offset
None Software list N/A N/A
KLV vlp N/A N/A N/A
Length Max Length Required Length
Variable Not Limited N/A

Resolution Special Values
N/A N/A

Required in LS? Optional Allowed in SDCC Pack? No Multiples Allowed? No

Software Value To KLV Value See Details
KLV Value To Software Value See Details
Example Software Value Example KLV Item (All Hex)
(0, 0, "VIS Nose Camera") Tag Len Value
(1, 0, "ACME VIS Model 123") 0312 0000 0F56 4953 204E 6F73 6520 4361
(2, 0, "ACME IR Model 456") 6D65 7261 1501 0012 4143 4D45 2056 4953
810A 3F
204D 6F64 656C 2031 3233 1402 0011 4143
[See Bullet Note Below] 4D45 2049 5220 4D6F 6465 6C20 3435 36
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(
            fields["pdf_example_software_value"],
            '(0, 0, "VIS Nose Camera") (1, 0, "ACME VIS Model 123") (2, 0, "ACME IR Model 456")',
        )
        self.assertEqual(
            fields["pdf_example_klv_item"],
            "Tag=810A Len=3F Value=0312 0000 0F56 4953 204E 6F73 6520 4361 6D65 7261 1501 0012 4143 4D45 2056 4953 204D 6F64 656C 2031 3233 1402 0011 4143 4D45 2049 5220 4D6F 6465 6C20 3435 36",
        )

    def test_parse_summary_block_preserves_formula_value_label_from_pdf(self):
        mod = load_module()
        summary = """8.5 Item 5: Platform Heading Angle
Description
Aircraft heading angle
Units Format Min Max Offset
Degrees Software float32 0 360
(°) KLV uint16 0 (2^16)-1 None
Length Max Length Required Length
2 2 2

Resolution Special Values
~5.5 millidegrees None

Required in LS? Optional Allowed in SDCC Pack? Yes Multiples Allowed? No

65535
Software Value To KLV Value KLVval = ( ) ∗ Soft Val
360
LSrange 360
KLV Value To Software Value Soft val = ( ) ∗ KLVint = ( ) ∗ KLVint
int range 65535
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_software_value_to_klv_value"], "KLVval = (65535/360) * Soft Val")
        self.assertEqual(fields["pdf_klv_value_to_software_value"], "Soft val = (360/65535) * KLVint")

    def test_parse_summary_block_keeps_numeric_software_example_separate_from_hex_value(self):
        mod = load_module()
        summary = """8.62 Item 62: Laser PRF Code
Description
Laser pulse repetition frequency code
Units Format Min Max Offset
None Software uint16 0 65535
KLV uint16 0 65535 N/A
Length Max Length Required Length
2 2 2

Resolution Special Values
1 None

Required in LS? Optional Allowed in SDCC Pack? No Multiples Allowed? No

Software Value To KLV Value KLVval = Soft Val
KLV Value To Software Value Soft val = KLVuint
Example Software Value Example KLV Item (All Hex)
Tag Len Value
1743
3E 02 06CF
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_example_software_value"], "1743")
        self.assertEqual(fields["pdf_example_klv_item"], "Tag=3E Len=02 Value=06CF")

    def test_parse_summary_block_keeps_single_token_string_example_separate_from_hex_value(self):
        mod = load_module()
        summary = """8.129 Item 129: Target ID
Description
Alpha-numeric identification of a target
Units Format Min Max Offset
None Software string N/A N/A
KLV utf8 N/A N/A N/A
Length Max Length Required Length
Variable 32 N/A

Resolution Special Values
N/A N/A

Required in LS? Optional Allowed in SDCC Pack? No Multiples Allowed? No

Software Value To KLV Value KLVval = Soft val
KLV Value To Software Value Soft val = KLVval
Example Software Value Example KLV Item (All Hex)
Tag Len Value
A123
8101 04 4131 3233
"""

        fields = mod.parse_summary_block(summary)

        self.assertEqual(fields["pdf_example_software_value"], "A123")
        self.assertEqual(fields["pdf_example_klv_item"], "Tag=8101 Len=04 Value=4131 3233")

    def test_generate_reference_writes_clean_143_item_markdown(self):
        mod = load_module()
        with tempfile.TemporaryDirectory() as tmpdir:
            output_path = Path(tmpdir) / "out.md"
            mod.generate_reference(PDF_PATH, output_path)
            rendered = output_path.read_text(encoding="utf-8")

        self.assertIn("- item_count: 143", rendered)
        self.assertEqual(rendered.count("\n## Item "), 143)
        self.assertIn("## Item 143 - Metadata Substream ID Pack", rendered)
        self.assertIn("- pdf_software_format: float64", rendered)
        self.assertIn("- pdf_klv_format: IMAPB", rendered)
        self.assertIn("- implementation_status: implemented_in_repo", rendered)
        self.assertNotIn("Motion Imagery Standards Board", rendered)
        self.assertNotIn("Figure 90:", rendered)
        self.assertNotIn("- format: ", rendered)
        self.assertIn("- pdf_example_klv_item: Tag=49 Len=- Value=N/A", rendered)
        self.assertIn('- pdf_special_values: 0x80000000 = "N/A (Off-Earth)" indicator', rendered)
        self.assertIn("- pdf_resolution: 2 bytes = 0.0625 degrees/second ; 3 bytes = 0.000244 degrees/second", rendered)
        self.assertIn('- pdf_example_software_value: (0, 0, "VIS Nose Camera") (1, 0, "ACME VIS Model 123") (2, 0, "ACME IR Model 456")', rendered)
        self.assertIn("- pdf_example_klv_item: Tag=810D Len=40 Value=0F00 0001 0340 71D8 9419 BDBF E708 9800 0F01 0002 0240 71D3 8819 BCCE 2408 FC00 0F02 7FFF 0140 71E3 0819 BF2C 1B07 D000 0F03 FFFE 0040 71E5 AF19 BF5A A709 6000", rendered)
        self.assertIn("- pdf_example_software_value: 1743", rendered)
        self.assertIn("- pdf_example_klv_item: Tag=3E Len=02 Value=06CF", rendered)
        self.assertIn("- pdf_example_software_value: A123", rendered)
        self.assertIn("- pdf_example_klv_item: Tag=8101 Len=04 Value=4131 3233", rendered)
        self.assertIn("- pdf_software_value_to_klv_value: KLVval = (65535/5000) * Soft Val", rendered)
        self.assertIn("- pdf_klv_value_to_software_value: Soft val = (5000/65535) * KLVuint", rendered)
        self.assertIn("- pdf_resolution: 2 bytes = 2 meters ; 3 bytes = 78.125 mm", rendered)
        self.assertIn("- pdf_special_values: None", rendered)
        self.assertIn("- pdf_example_klv_item: Tag=810F Len=11 Value=008D C4F4 623E A25A 859C 5D0A F0C9 5E8C 39", rendered)
        self.assertIn("- pdf_example_software_value: April 16, 1995 13:44:54 (798039894000000)", rendered)
        self.assertIn("- pdf_example_klv_item: Tag=48 Len=08 Value=0x0002 D5D0 2466 0180", rendered)
        self.assertIn("- pdf_example_klv_item: Tag=5E Len=22 Value=0170 F592 F023 7336 4AF8 AA91 62C0 0F2E B2DA 16B7 4341 0008 41A0 BE36 5B5A B96A 3645", rendered)
        self.assertIn("- pdf_units: Revolutions Per Minute (RPM)", rendered)
        self.assertIn("- pdf_klv_format: uint", rendered)
        self.assertIn("- pdf_example_klv_item: Tag=8102 Len=18 Value=0B40 6BC2 0919 BDA5 5407 0E00 0B40 783C B819 A292 7407 C600", rendered)
        self.assertIn('- pdf_example_software_value: (1, 1, 1, 3,([0,0, 0, 1], 3), "Harpoon") (1, 1, 2, 2,([1,1, 1, 1], 4), "Hellfire") (1, 2, 1, 1,([0,0, 0, 0], 3), "GBU-15")', rendered)
        self.assertIn("- pdf_example_klv_item: Tag=810C Len=2D Value=0E01 0101 0382 0307 4861 7270 6F6F 6E0F 0101 0202 9E04 0848 656C 6C66 6972 650C 0102 0101 0306 4742 552D 3135", rendered)


if __name__ == "__main__":
    unittest.main()

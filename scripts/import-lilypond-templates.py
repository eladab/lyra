#!/usr/bin/env python3
"""Import the template appendix from a saved official 2.26 Learning Manual.

Usage: python3 scripts/import-lilypond-templates.py learning-big-page.html
The first complete, displayed example in each template topic is retained.
No network access or LilyPond execution occurs here or in the startup picker.
"""

import json
import re
import sys
from html.parser import HTMLParser
from pathlib import Path

BASE = "https://lilypond.org/doc/v2.26/Documentation/learning/"
ALIASES = {
    "satb-template": "choir choral four part soprano alto tenor bass piano accompaniment",
    "ssaattbb-template": "choir choral eight part double choir soprano alto tenor bass",
    "notes-only": "simple basic melody solo instrument one staff stave",
    "notes-and-lyrics": "voice vocal song singer text words melody",
    "notes-and-chords": "lead sheet guitar chord names harmony melody",
    "notes-lyrics-and-chords": "lead sheet song voice vocal guitar chord names words",
    "solo-piano": "keyboard grand staff two staves hands",
    "piano-and-melody-with-lyrics": "voice vocal song singer accompaniment keyboard",
    "piano_002dcentered-lyrics": "keyboard voice vocal words between staves",
    "string-quartet": "chamber ensemble violin violins viola cello violoncello full score",
    "string-quartet-parts": "chamber ensemble violin violins viola cello separate individual parts",
    "satb-vocal-score": "choir choral four part soprano alto tenor bass",
    "satb-vocal-score-and-automatic-piano-reduction": "choir choral four part keyboard accompaniment",
    "satb-with-aligned-contexts": "choir choral four part soprano alto tenor bass lyrics",
    "satb-on-four-staves": "choir choral four part separate staves soprano alto tenor bass",
    "solo-verse-and-two_002dpart-refrain": "voice vocal song duet chorus",
    "hymn-tunes": "choir church choral hymnals sacred chorale",
    "psalms": "chant psalm sacred church voice vocal",
    "orchestra-choir-and-piano": "orchestral symphonic ensemble choral keyboard full score conductor",
    "transcription-of-mensural-music": "ancient early renaissance medieval historical old notation",
    "gregorian-transcription-template": "ancient early plainchant plainsong chant historical medieval",
    "jazz-combo": "band ensemble jazz leadsheet lead sheet saxophone sax trumpet bass drums",
}


class Templates(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.heading = None
        self.heading_text = ""
        self.anchor = ""
        self.active = False
        self.category = ""
        self.current = None
        self.code = None
        self.templates = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "div" and attrs.get("class") == "appendixsubsec":
            self.anchor = attrs["id"]
        if tag in ("h2", "h3") or (tag == "h4" and attrs.get("class") == "appendixsubsec"):
            self.heading, self.heading_text = tag, ""
        if self.current and not self.current["Code"] and tag == "pre" and attrs.get("class") == "verbatim":
            self.code = ""

    def handle_data(self, text):
        if self.heading:
            self.heading_text += text
        if self.code is not None:
            self.code += text

    def handle_endtag(self, tag):
        if tag == self.heading:
            title = " ".join(self.heading_text.split())
            if tag == "h2":
                self.active = title == "Appendix A Templates"
                self.current = None
            elif self.active and tag == "h3":
                self.category = re.sub(r"^A\.\d+\s+", "", title)
                self.category = re.sub(r" templates$", "", self.category)
                self.current = None
            elif self.active:
                name = re.sub(r"^A\.\d+\.\d+\s+", "", title)
                self.current = dict(ID=self.anchor, Category=self.category, Name=name,
                                    Source=BASE + self.anchor, Aliases=ALIASES[self.anchor], Code="")
                self.templates.append(self.current)
            self.heading = None
        if tag == "pre" and self.code is not None:
            self.current["Code"] = self.code.strip("\n") + "\n"
            self.code = None


parser = Templates()
parser.feed(Path(sys.argv[1]).read_text())
assert len(parser.templates) == 22, "Unexpected template inventory; inspect the source manual"
for template in parser.templates:
    assert template["Code"], f"No complete example in {template['Name']}"
    header = (f"% {template['Category']} - {template['Name']}\n"
              f"% Source: {template['Source']}\n"
              "% LilyPond Learning Manual, copyright 1998-2026 its authors.\n"
              "% GNU FDL 1.1 or later; no Invariant Sections.\n"
              "% License: https://www.gnu.org/licenses/old-licenses/fdl-1.1.html\n")
    template["Code"] = header + '\\version "2.26.0"\n\n' + template["Code"]
output = Path(__file__).resolve().parents[1] / "internal/scoretemplate/templates.json"
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text(json.dumps(parser.templates, ensure_ascii=False, indent=2) + "\n")
print(f"Imported {len(parser.templates)} templates into {output}")

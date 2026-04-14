#!/usr/bin/env python3

import argparse
import decimal
import json
import re
import subprocess
import tempfile
from pathlib import Path


NULL = "null"
ITEM_HEADING_RE = re.compile(r"^\s*Item\s+(\d+):\s+(.+?)\s*$")
DETAILS_HEADING_RE = re.compile(r"^\s*8\.\d+(?:\.\d+){0,2}\s+Details\s*$")
SECTION_ONLY_RE = re.compile(r"^\s*8\.\d+(?:\.\d+)?\s*$")

ITEM_FIXUPS: dict[int, dict[str, object]] = {
    72: {
        "pdf_example_software_value": "April 16, 1995 13:44:54 (798039894000000)",
        "pdf_example_klv_item": "Tag=48 Len=08 Value=0x0002 D5D0 2466 0180",
    },
    94: {
        "pdf_example_software_value": "0170:F592-F023-7336-4AF8-AA91-62C0-0F2E-B2DA/16B7-4341-0008-41A0-BE36-5B5A-B96A-3645:D3",
        "pdf_example_klv_item": "Tag=5E Len=22 Value=0170 F592 F023 7336 4AF8 AA91 62C0 0F2E B2DA 16B7 4341 0008 41A0 BE36 5B5A B96A 3645",
    },
    111: {
        "pdf_units": "Revolutions Per Minute (RPM)",
        "pdf_klv_format": "uint",
        "pdf_klv_min": "0",
        "pdf_klv_max": "(2^32)-1",
        "pdf_offset": "N/A",
    },
    130: {
        "pdf_example_software_value": "(38.841859, -77.036784, 3), (38.939353, -77.459811, 95)",
        "pdf_example_klv_item": "Tag=8102 Len=18 Value=0B40 6BC2 0919 BDA5 5407 0E00 0B40 783C B819 A292 7407 C600",
    },
    140: {
        "pdf_example_software_value": '(1, 1, 1, 3,([0,0, 0, 1], 3), "Harpoon") (1, 1, 2, 2,([1,1, 1, 1], 4), "Hellfire") (1, 2, 1, 1,([0,0, 0, 0], 3), "GBU-15")',
        "pdf_example_klv_item": "Tag=810C Len=2D Value=0E01 0101 0382 0307 4861 7270 6F6F 6E0F 0101 0202 9E04 0848 656C 6C66 6972 650C 0102 0101 0306 4742 552D 3135",
    },
}


def split_columns(line: str) -> list[str]:
    return [part.strip() for part in re.split(r"\s{2,}", line.strip()) if part.strip()]


def split_tokens(line: str) -> list[str]:
    return line.strip().split()


def is_hexish_token(token: str) -> bool:
    return bool(re.fullmatch(r"[0-9A-Fa-f]+|-", token))


def is_hexish_sequence(text: str) -> bool:
    return bool(re.fullmatch(r"[0-9A-Fa-f][0-9A-Fa-f ]*[0-9A-Fa-f]|-", text.strip()))


def clean_text(text: str) -> str:
    text = text.replace("\x0c", "\n")
    text = re.sub(r"[ \t]+", " ", text)
    text = re.sub(r" *\n *", "\n", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def strip_example_noise(text: str) -> str:
    text = re.sub(r"\bTag\s+Len\s+Value\b", " ", text)
    text = re.sub(r"\[\s*See Bullet Note Below\s*\]", " ", text)
    return clean_text(text)


def split_example_software_and_hex(text: str) -> tuple[str | None, str | None]:
    cleaned = strip_example_noise(text)
    if not cleaned:
        return None, None
    if " " not in cleaned:
        return cleaned, None
    if re.fullmatch(r"[+-]?\d[\d,]*(?:\.\d+)?", cleaned):
        return cleaned, None
    if is_hexish_sequence(cleaned):
        return None, cleaned

    match = re.match(r"^(.*?\S)\s+([0-9A-Fa-f]{2,}(?: [0-9A-Fa-f]{2,})*)$", cleaned)
    if match:
        prefix = clean_text(match.group(1))
        if re.search(r'[(),"\'-]', prefix):
            return prefix, clean_text(match.group(2))
    return cleaned, None


def is_layout_noise(line: str) -> bool:
    stripped = line.strip()
    if not stripped:
        return False
    if stripped == "ST 0601.19 UAS Datalink Local Set":
        return True
    if "Motion Imagery Standards Board" in stripped and re.search(r"\b02 March 2023\b", stripped):
        return True
    return False


def strip_layout_noise_lines(lines: list[str]) -> list[str]:
    return [line for line in lines if not is_layout_noise(line)]


def find_line_index(lines: list[str], marker: str) -> int:
    for i, line in enumerate(lines):
        if marker in line:
            return i
    return -1


def next_nonempty(lines: list[str], start: int) -> tuple[int, str]:
    for i in range(start, len(lines)):
        if lines[i].strip():
            return i, lines[i]
    return -1, ""


def collect_until(lines: list[str], start: int, stop_markers: tuple[str, ...]) -> list[str]:
    out = []
    for line in lines[start:]:
        stripped = line.strip()
        if not stripped:
            if out:
                break
            continue
        if any(marker in line for marker in stop_markers):
            break
        if stripped.startswith("•"):
            break
        out.append(stripped)
    return out


def parse_example_klv_item(lines: list[str]) -> str | None:
    for line in lines:
        cols = split_columns(line)
        if len(cols) >= 3 and is_hexish_token(cols[0]) and is_hexish_token(cols[1]):
            return f"Tag={cols[0]} Len={cols[1]} Value={' '.join(cols[2:])}"
    if len(lines) >= 2:
        first_cols = split_columns(lines[0])
        second_cols = split_columns(lines[1])
        if len(first_cols) == 1 and len(second_cols) >= 2 and is_hexish_token(second_cols[0]) and is_hexish_token(second_cols[1]):
            remainder = " ".join([first_cols[0], *[line.strip() for line in lines[2:]]]).strip()
            return f"Tag={second_cols[0]} Len={second_cols[1]} Value={remainder}"
    joined = clean_text("\n".join(lines))
    return joined or None


def parse_resolution_and_special(lines: list[str], header_idx: int) -> tuple[str | None, str | None]:
    resolution_parts = []
    special_parts = []
    for line in lines[header_idx + 1 :]:
        if not line.strip():
            if resolution_parts or special_parts:
                break
            continue
        if "Required in LS?" in line:
            break
        cols = split_columns(line)
        if not cols:
            continue
        if len(cols) == 1:
            if cols[0] in {"None", "N/A"} and resolution_parts and not special_parts:
                special_parts.append(cols[0])
                continue
            if resolution_parts and not re.search(r"\d|=", cols[0]):
                resolution_parts[-1] = clean_text(f"{resolution_parts[-1]} {cols[0]}")
                continue
            resolution_parts.append(cols[0])
            continue
        resolution_parts.append(cols[0])
        special_parts.append(" ".join(cols[1:]))
    resolution = " ; ".join(resolution_parts) if resolution_parts else None
    special = " ; ".join(special_parts) if special_parts else None
    return resolution, special


def parse_summary_block(summary: str) -> dict[str, object]:
    lines = summary.splitlines()
    data: dict[str, object] = {
        "pdf_description": None,
        "pdf_units": None,
        "pdf_software_format": None,
        "pdf_software_min": None,
        "pdf_software_max": None,
        "pdf_klv_format": None,
        "pdf_klv_min": None,
        "pdf_klv_max": None,
        "pdf_offset": None,
        "pdf_length": None,
        "pdf_max_length": None,
        "pdf_required_length": None,
        "pdf_resolution": None,
        "pdf_special_values": None,
        "pdf_required_in_ls": None,
        "pdf_allowed_in_sdcc_pack": None,
        "pdf_multiples_allowed": None,
        "pdf_software_value_to_klv_value": None,
        "pdf_klv_value_to_software_value": None,
        "pdf_example_software_value": None,
        "pdf_example_klv_item": None,
        "pdf_notes": [],
    }

    desc_idx = find_line_index(lines, "Description")
    if desc_idx != -1:
        desc_lines = collect_until(lines, desc_idx + 1, ("Units",))
        data["pdf_description"] = clean_text("\n".join(desc_lines)) or None

    units_idx = find_line_index(lines, "Units")
    if units_idx != -1:
        _, row1 = next_nonempty(lines, units_idx + 1)
        _, row2 = next_nonempty(lines, units_idx + 2)
        cols1 = split_columns(row1)
        cols2 = split_columns(row2)
        if "Software" in cols1:
            sidx = cols1.index("Software")
            units = cols1[:sidx]
            software = cols1[sidx + 1 :]
            data["pdf_units"] = clean_text(" ".join(units)) if units else None
            if software:
                data["pdf_software_format"] = software[0]
            if len(software) > 1:
                data["pdf_software_min"] = software[1]
            if len(software) > 2:
                data["pdf_software_max"] = software[2]
        if "KLV" in cols2:
            kidx = cols2.index("KLV")
            units_tail = cols2[:kidx]
            existing_units = data["pdf_units"] or ""
            units = clean_text(" ".join([existing_units, *units_tail])) if units_tail else clean_text(str(existing_units))
            data["pdf_units"] = units or None
            klv = cols2[kidx + 1 :]
            if klv:
                data["pdf_klv_format"] = klv[0]
            if len(klv) > 1:
                data["pdf_klv_min"] = klv[1]
            if len(klv) > 2:
                data["pdf_klv_max"] = klv[2]
            if len(klv) > 3:
                data["pdf_offset"] = klv[3]
        else:
            row1_tokens = split_tokens(row1)
            row2_tokens = split_tokens(row2)
            if "Software" in row1_tokens:
                sidx = row1_tokens.index("Software")
                data["pdf_units"] = clean_text(" ".join(row1_tokens[:sidx])) or None
                tail = row1_tokens[sidx + 1 :]
                if len(tail) >= 1:
                    data["pdf_software_format"] = tail[0]
                if len(tail) >= 2:
                    data["pdf_software_min"] = tail[1]
                if len(tail) >= 3:
                    data["pdf_software_max"] = tail[2]
            if "KLV" in row2_tokens:
                kidx = row2_tokens.index("KLV")
                units_tail = row2_tokens[:kidx]
                existing_units = data["pdf_units"] or ""
                merged_units = clean_text(" ".join([existing_units, *units_tail]))
                data["pdf_units"] = merged_units or None
                tail = row2_tokens[kidx + 1 :]
                if len(tail) >= 1:
                    data["pdf_klv_format"] = tail[0]
                if len(tail) >= 2:
                    data["pdf_klv_min"] = tail[1]
                if len(tail) >= 3:
                    data["pdf_klv_max"] = tail[2]
                if len(tail) >= 4:
                    data["pdf_offset"] = tail[3]

    length_idx = find_line_index(lines, "Required Length")
    if length_idx != -1:
        _, row = next_nonempty(lines, length_idx + 1)
        cols = split_columns(row)
        if cols:
            data["pdf_length"] = cols[0]
        if len(cols) > 1:
            data["pdf_max_length"] = cols[1]
        if len(cols) > 2:
            data["pdf_required_length"] = cols[2]

    resolution_idx = find_line_index(lines, "Special Values")
    if resolution_idx != -1:
        resolution, special = parse_resolution_and_special(lines, resolution_idx)
        data["pdf_resolution"] = resolution
        data["pdf_special_values"] = special

    required_idx = find_line_index(lines, "Required in LS?")
    if required_idx != -1:
        required_line = clean_text(lines[required_idx])
        match = re.search(
            r"Required in LS\?\s+(.*?)\s+Allowed in SDCC Pack\?\s+(.*?)\s+Multiples Allowed\?\s+(.*)",
            required_line,
        )
        if match:
            data["pdf_required_in_ls"] = match.group(1).strip()
            data["pdf_allowed_in_sdcc_pack"] = match.group(2).strip()
            data["pdf_multiples_allowed"] = match.group(3).strip()

    sw_formula_idx = find_line_index(lines, "Software Value To KLV Value")
    if sw_formula_idx != -1:
        line = lines[sw_formula_idx].split("Software Value To KLV Value", 1)[1].strip()
        tail = collect_until(lines, sw_formula_idx + 1, ("KLV Value To Software Value", "Example Software Value"))
        data["pdf_software_value_to_klv_value"] = clean_text(" ".join([line, *tail])) or None

    klv_formula_idx = find_line_index(lines, "KLV Value To Software Value")
    if klv_formula_idx != -1:
        line = lines[klv_formula_idx].split("KLV Value To Software Value", 1)[1].strip()
        tail = collect_until(lines, klv_formula_idx + 1, ("Example Software Value",))
        data["pdf_klv_value_to_software_value"] = clean_text(" ".join([line, *tail])) or None

    example_idx = find_line_index(lines, "Example Software Value")
    if example_idx != -1:
        block_lines = []
        blank_count = 0
        for line in lines[example_idx + 1 :]:
            stripped = line.strip()
            if not stripped:
                if block_lines:
                    blank_count += 1
                    if blank_count >= 2:
                        break
                    continue
                continue
            blank_count = 0
            if stripped.startswith("•"):
                break
            block_lines.append(stripped)

        def parse_tag_len_candidate(line: str):
            cols = split_tokens(line)
            if len(cols) >= 2 and is_hexish_token(cols[0]) and is_hexish_token(cols[1]):
                return cols[0], cols[1], cols[2:]
            return None

        tag_line_idx = None
        for i, line in enumerate(block_lines):
            if re.fullmatch(r"Tag\s+Len\s+Value", line):
                continue
            if parse_tag_len_candidate(line):
                tag_line_idx = i
                break

        software_lines = block_lines[: tag_line_idx if tag_line_idx is not None else len(block_lines)]
        klv_lines = block_lines[tag_line_idx:] if tag_line_idx is not None else []

        software_parts = []
        klv_prefix = []
        for line in software_lines:
            if re.fullmatch(r"Tag\s+Len\s+Value", line):
                continue
            software_fragment, hex_fragment = split_example_software_and_hex(line.strip())
            if software_fragment:
                software_parts.append(software_fragment)
            if hex_fragment:
                klv_prefix.append(hex_fragment)
            if software_fragment or hex_fragment:
                continue

        software_example = clean_text(" ".join(software_parts)) or None

        parsed_klv = None
        if klv_lines:
            first = parse_tag_len_candidate(klv_lines[0])
            if first:
                tag, length, rest = first
                value_parts = []
                value_parts.extend(klv_prefix)
                value_parts.extend(rest)
                for extra in klv_lines[1:]:
                    _, extra_hex = split_example_software_and_hex(extra.strip())
                    cleaned_extra = extra_hex or strip_example_noise(extra.strip())
                    if cleaned_extra:
                        value_parts.append(cleaned_extra)
                parsed_klv = f"Tag={tag} Len={length} Value={clean_text(' '.join(value_parts)) or 'N/A'}"
        if parsed_klv is None:
            parsed_klv = parse_example_klv_item(klv_lines if klv_lines else klv_prefix)

        data["pdf_example_software_value"] = software_example
        data["pdf_example_klv_item"] = parsed_klv

    notes = []
    current_note = None
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("•"):
            if current_note is not None:
                notes.append(current_note)
            current_note = stripped.lstrip("•").strip()
        elif current_note is not None and stripped and not any(
            marker in stripped
            for marker in (
                "Details",
                "Required in LS?",
                "Software Value To KLV Value",
                "KLV Value To Software Value",
                "Example Software Value",
            )
        ):
            current_note += " " + stripped
    if current_note is not None:
        notes.append(current_note)
    data["pdf_notes"] = notes

    maybe_reconstruct_formula(data)
    return data


def parse_decimal(text: str | None) -> decimal.Decimal | None:
    if not text:
        return None
    normalized = text.replace(",", "").replace("−", "-").strip()
    if normalized in {"N/A", "None"}:
        return None
    try:
        return decimal.Decimal(normalized)
    except decimal.InvalidOperation:
        return None


def decimal_str(value: decimal.Decimal) -> str:
    normalized = value.normalize()
    as_str = format(normalized, "f")
    if "." in as_str:
        as_str = as_str.rstrip("0").rstrip(".")
    return as_str


def maybe_reconstruct_formula(data: dict[str, object]) -> None:
    soft_formula = data.get("pdf_software_value_to_klv_value")
    inv_formula = data.get("pdf_klv_value_to_software_value")
    if not isinstance(soft_formula, str) or not re.search(r"\(\s*\)", soft_formula):
        return
    klv_format = data.get("pdf_klv_format")
    if not isinstance(klv_format, str):
        return

    soft_min = parse_decimal(data.get("pdf_software_min"))
    soft_max = parse_decimal(data.get("pdf_software_max"))
    offset = parse_decimal(data.get("pdf_offset"))
    if offset is None:
        offset = decimal.Decimal("0")
    if soft_min is None or soft_max is None:
        return

    if klv_format.startswith("uint"):
        bits = int(klv_format[4:])
        int_range = decimal.Decimal((2**bits) - 1)
        value_label = "KLVuint"
    elif klv_format.startswith("int"):
        bits = int(klv_format[3:])
        int_range = decimal.Decimal((2**bits) - 2)
        value_label = "KLVint"
    else:
        return
    if isinstance(inv_formula, str):
        label_match = re.search(r"\b(KLV(?:u?int|val))\b", inv_formula)
        if label_match:
            value_label = label_match.group(1)

    ls_range = soft_max - soft_min
    if ls_range == 0:
        return

    numerator = decimal_str(int_range)
    denominator = decimal_str(ls_range)
    if offset == 0:
        soft_term = "Soft Val"
        inverse_offset = ""
    else:
        soft_term = f"(Soft Val + {decimal_str(-offset)})"
        inverse_offset = f" - {decimal_str(-offset)}"

    data["pdf_software_value_to_klv_value"] = f"KLVval = ({numerator}/{denominator}) * {soft_term}"
    data["pdf_klv_value_to_software_value"] = f"Soft val = ({denominator}/{numerator}) * {value_label}{inverse_offset}"


def apply_item_fixups(item: dict[str, object]) -> None:
    for key, value in ITEM_FIXUPS.get(int(item["item"]), {}).items():
        item[key] = value


def clean_details(text: str) -> str:
    cleaned_lines = []
    for raw in strip_layout_noise_lines(text.splitlines()):
        line = raw.strip()
        if not line:
            cleaned_lines.append("")
            continue
        if line.startswith("Figure "):
            continue
        if set(line) <= {"\ufeff"}:
            continue
        line = re.sub(r"\bFigure\s+\d+\b", "the figure", line)
        cleaned_lines.append(line)

    text = clean_text("\n".join(cleaned_lines))
    paragraphs = [p for p in text.split("\n\n") if p.strip()]
    return "\n\n".join(paragraphs)


def extract_layout_text(pdf_path: Path) -> str:
    with tempfile.NamedTemporaryFile(suffix=".txt") as tmp:
        subprocess.run(
            ["pdftotext", "-layout", str(pdf_path), tmp.name],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        return Path(tmp.name).read_text(encoding="utf-8", errors="replace")


def extract_item_sections(layout_text: str) -> list[dict[str, object]]:
    end = layout_text.rfind("Appendix A")
    body = layout_text[:end if end != -1 else None]
    lines = body.splitlines()

    headings = []
    started = False
    pending_section = None
    for idx, line in enumerate(lines):
        stripped = line.strip()
        if SECTION_ONLY_RE.fullmatch(stripped):
            pending_section = stripped
            continue
        inline_match = re.match(r"^\s*(8\.\d+(?:\.\d+)?)\s+Item\s+(\d+):\s+(.+?)\s*$", stripped)
        if inline_match:
            item = int(inline_match.group(2))
            section = inline_match.group(1)
            if not started and not (item == 1 and section == "8.1"):
                pending_section = None
                continue
            started = True
            headings.append((idx, item, inline_match.group(3).strip(), section))
            pending_section = None
            continue
        item_match = ITEM_HEADING_RE.match(stripped)
        if item_match:
            item = int(item_match.group(1))
            if not started and not (item == 1 and pending_section == "8.1"):
                pending_section = None
                continue
            started = True
            headings.append((idx, item, item_match.group(2).strip(), pending_section))
            pending_section = None
            if item == 143:
                break

    items = []
    for i, (start_idx, item, name, section) in enumerate(headings):
        next_idx = headings[i + 1][0] if i + 1 < len(headings) else len(lines)
        section_lines = lines[start_idx:next_idx]

        details_idx = None
        for j, line in enumerate(section_lines):
            if DETAILS_HEADING_RE.match(line.strip()):
                details_idx = j
                break

        if details_idx is None:
            summary_lines = section_lines
            details_lines = []
        else:
            summary_lines = section_lines[:details_idx]
            details_lines = section_lines[details_idx + 1 :]

        summary_lines = strip_layout_noise_lines(summary_lines)
        raw_summary = "\n".join(summary_lines)
        summary_text = clean_text(raw_summary)
        parsed = parse_summary_block(raw_summary)
        items.append(
            {
                "item": item,
                "tag": item,
                "name": name,
                "section": section,
                "pdf_summary_block": summary_text,
                "pdf_details": clean_details("\n".join(details_lines)),
                **parsed,
            }
        )
        apply_item_fixups(items[-1])

    return items


def load_implemented_tags(v19_path: Path) -> set[int]:
    code = v19_path.read_text(encoding="utf-8", errors="replace")
    tags = {int(n) for n in re.findall(r"tags\[(\d+)\]", code)}
    tags.update(int(n) for n in re.findall(r"cornerTag\((\d+)", code))
    return tags


def render_value(value: object) -> str:
    if value in (None, "", []):
        return NULL
    if isinstance(value, list):
        return json.dumps(value, ensure_ascii=False)
    return clean_text(str(value)).replace("\n", " ")


def render_block(label: str, value: str) -> list[str]:
    if not value:
        return [f"- {label}: {NULL}"]
    return [f"- {label}:", "```text", value, "```"]


def render_markdown(items: list[dict[str, object]], implemented_tags: set[int]) -> str:
    out = [
        "# ST0601.19 Tag Reference",
        "",
        "- source_pdf: `internal/klv/specs/st0601/ST0601.19.pdf`",
        "- generated_with: `internal/klv/specs/st0601/generate_reference.py`",
        "- purpose: LLM-friendly per-item ST0601.19 reference with atomic PDF fields and sanitized prose details",
        f"- item_count: {len(items)}",
        "- null_sentinel: `null`",
        "",
    ]

    scalar_fields = [
        "item",
        "tag",
        "name",
        "section",
        "pdf_description",
        "pdf_units",
        "pdf_software_format",
        "pdf_software_min",
        "pdf_software_max",
        "pdf_klv_format",
        "pdf_klv_min",
        "pdf_klv_max",
        "pdf_offset",
        "pdf_length",
        "pdf_max_length",
        "pdf_required_length",
        "pdf_resolution",
        "pdf_special_values",
        "pdf_required_in_ls",
        "pdf_allowed_in_sdcc_pack",
        "pdf_multiples_allowed",
        "pdf_software_value_to_klv_value",
        "pdf_klv_value_to_software_value",
        "pdf_example_software_value",
        "pdf_example_klv_item",
    ]

    for item in items:
        out.append(f"## Item {item['item']} - {item['name']}")
        out.append("")
        for field in scalar_fields:
            out.append(f"- {field}: {render_value(item.get(field))}")
        out.append(f"- implementation_status: {'implemented_in_repo' if item['item'] in implemented_tags else 'not_implemented_in_repo'}")
        out.append(f"- pdf_notes: {render_value(item.get('pdf_notes'))}")
        out.extend(render_block("pdf_summary_block", str(item.get("pdf_summary_block") or "")))
        out.extend(render_block("pdf_details", str(item.get("pdf_details") or "")))
        out.append("")

    return "\n".join(out) + "\n"


def generate_reference(pdf_path: Path, output_path: Path) -> None:
    layout_text = extract_layout_text(pdf_path)
    items = extract_item_sections(layout_text)
    implemented_tags = load_implemented_tags(Path(__file__).with_name("v19.go"))
    output_path.write_text(render_markdown(items, implemented_tags), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate the ST0601.19 markdown reference from the PDF.")
    parser.add_argument("--pdf", type=Path, default=Path(__file__).with_name("ST0601.19.pdf"))
    parser.add_argument("--out", type=Path, default=Path(__file__).with_name("ST0601.19.tags.md"))
    args = parser.parse_args()
    generate_reference(args.pdf, args.out)


if __name__ == "__main__":
    main()

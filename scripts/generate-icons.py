#!/usr/bin/env python3
"""
Generate Chrome extension icons from a single source image.

Usage:
    python scripts/generate-icons.py <source_image>

Example:
    python scripts/generate-icons.py assets/icon-source.png

Outputs:
    extension/icons/icon16.png
    extension/icons/icon48.png
    extension/icons/icon128.png

Requirements:
    pip install Pillow
"""

import sys
import os
from pathlib import Path

from PIL import Image

# Chrome extension required icon sizes
ICON_SIZES = [16, 48, 128]
OUTPUT_DIR = Path("extension/icons")


def generate_icons(source_path: str) -> None:
    source = Path(source_path)
    if not source.exists():
        print(f"Error: File not found: {source}")
        sys.exit(1)

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    with Image.open(source) as img:
        # Convert to RGBA if not already (ensures transparency support)
        if img.mode != "RGBA":
            img = img.convert("RGBA")

        for size in ICON_SIZES:
            # Use LANCZOS for high-quality downscaling
            resized = img.resize((size, size), Image.LANCZOS)
            output_path = OUTPUT_DIR / f"icon{size}.png"
            resized.save(output_path, "PNG")
            print(f"  Generated: {output_path}")

    print(f"\nDone! {len(ICON_SIZES)} icons written to {OUTPUT_DIR}/")


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        print(f"Usage: python {sys.argv[0]} <source_image>")
        sys.exit(1)

    source = sys.argv[1]
    print(f"Generating Chrome extension icons from: {source}")
    generate_icons(source)


if __name__ == "__main__":
    main()

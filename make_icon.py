from __future__ import annotations

import sys
from pathlib import Path

try:
    from PIL import Image
except ImportError as exc:
    raise SystemExit("Pillow is required: python -m pip install pillow") from exc


def main() -> None:
    src = Path(sys.argv[1] if len(sys.argv) > 1 else "assets/Typora-logo.png")
    out = Path(sys.argv[2] if len(sys.argv) > 2 else "Typora.ico")
    sizes = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]

    image = Image.open(src).convert("RGBA")
    image.save(out, format="ICO", sizes=sizes)
    print(f"Generated {out} from {src}")


if __name__ == "__main__":
    main()

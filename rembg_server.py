"""
Persistent FastAPI background-removal server.

Model is selected via REMBG_MODEL env variable:
    birefnet-general-lite  — default, fast, ~60 MB,  good for local dev / weak CPU
    birefnet-portrait      — production, ~400 MB, best quality for human subjects

Install:
    pip install fastapi uvicorn rembg pillow          # CPU
    pip install fastapi uvicorn "rembg[gpu]" pillow   # NVIDIA GPU (Linux/Windows)

Run:
    python rembg_server.py
    REMBG_MODEL=birefnet-portrait python rembg_server.py   # production

Endpoint:
    POST /remove-bg   multipart/form-data field: "file"  → PNG bytes
    GET  /health                                         → {"status": "ok", "model": "..."}

Notes:
    - First run downloads the model from HuggingFace automatically.
    - alpha_matting=True is enabled for smoother hair/edge blending.
    - Images larger than MAX_DIM are downscaled before inference and the
      result is upscaled back, keeping quality high while limiting memory use.
"""

import io
import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, File, HTTPException, UploadFile
from fastapi.responses import Response
from PIL import Image
from rembg import new_session, remove

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)

# birefnet-general-lite on local (fast, light), birefnet-portrait on prod (best quality).
MODEL_NAME = os.getenv("REMBG_MODEL", "birefnet-general-lite")

# Resize input to this maximum dimension before inference.
# Keeps memory use predictable; result is rescaled back to original size.
MAX_DIM = 1024

# Global session — model loaded once at startup.
_session = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _session
    logger.info("Loading %s model (first run downloads ~400 MB)...", MODEL_NAME)
    _session = new_session(MODEL_NAME)
    logger.info("Model ready.")
    yield
    logger.info("Shutting down rembg server.")


app = FastAPI(title="rembg-server", lifespan=lifespan)


def _preprocess(data: bytes) -> tuple[bytes, tuple[int, int]]:
    """Downscale image to MAX_DIM if needed. Returns (bytes, original_size)."""
    img = Image.open(io.BytesIO(data)).convert("RGBA")
    original_size = img.size  # (width, height)
    w, h = original_size
    if max(w, h) > MAX_DIM:
        scale = MAX_DIM / max(w, h)
        new_w, new_h = int(w * scale), int(h * scale)
        img = img.resize((new_w, new_h), Image.LANCZOS)
        buf = io.BytesIO()
        img.save(buf, format="PNG")
        return buf.getvalue(), original_size
    return data, original_size


def _postprocess(result_bytes: bytes, original_size: tuple[int, int]) -> bytes:
    """Upscale result back to original_size if it was downscaled."""
    result = Image.open(io.BytesIO(result_bytes)).convert("RGBA")
    if result.size != original_size:
        result = result.resize(original_size, Image.LANCZOS)
    buf = io.BytesIO()
    result.save(buf, format="PNG")
    return buf.getvalue()


@app.get("/health")
def health():
    return {"status": "ok", "model": MODEL_NAME}


@app.post("/remove-bg")
async def remove_bg(file: UploadFile = File(...)):
    if _session is None:
        raise HTTPException(status_code=503, detail="Model not ready")

    data = await file.read()
    if len(data) > 20 * 1024 * 1024:
        raise HTTPException(status_code=400, detail="file too large (max 20MB)")

    try:
        data, original_size = _preprocess(data)
        result = remove(
            data,
            session=_session,
            alpha_matting=True,
            alpha_matting_foreground_threshold=240,
            alpha_matting_background_threshold=10,
            alpha_matting_erode_size=10,
        )
        result = _postprocess(result, original_size)
    except Exception as e:
        logger.error("rembg inference failed: %s", e)
        raise HTTPException(status_code=500, detail=f"inference failed: {e}")

    return Response(content=result, media_type="image/png")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8004, log_level="info")

"""
Persistent FastAPI background-removal server using BRIA RMBG 1.4.
Model is loaded once at startup and kept in memory → fast inference.

Install:
    pip install fastapi uvicorn rembg[gpu] pillow

Run:
    python rembg_server.py
    # or:
    uvicorn rembg_server:app --host 127.0.0.1 --port 8004

Endpoint:
    POST /remove-bg   multipart/form-data field: "file"  → PNG bytes
"""

import io
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, File, HTTPException, UploadFile
from fastapi.responses import Response
from rembg import new_session, remove

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)

# Global session — model loaded once
_session = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _session
    logger.info("Loading isnet-general-use model...")
    _session = new_session("isnet-general-use")
    logger.info("Model ready.")
    yield
    logger.info("Shutting down rembg server.")


app = FastAPI(title="rembg-server", lifespan=lifespan)


@app.get("/health")
def health():
    return {"status": "ok", "model": "isnet-general-use"}


@app.post("/remove-bg")
async def remove_bg(file: UploadFile = File(...)):
    if _session is None:
        raise HTTPException(status_code=503, detail="Model not ready")

    data = await file.read()
    if len(data) > 20 * 1024 * 1024:
        raise HTTPException(status_code=400, detail="file too large (max 20MB)")

    try:
        result = remove(data, session=_session)
    except Exception as e:
        logger.error("rembg inference failed: %s", e)
        raise HTTPException(status_code=500, detail=f"inference failed: {e}")

    return Response(content=result, media_type="image/png")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8004, log_level="info")

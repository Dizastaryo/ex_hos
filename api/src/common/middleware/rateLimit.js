const rateMap = new Map();

function rateLimit({ windowMs = 60000, max = 60 } = {}) {
  return (req, res, next) => {
    const key = req.ip + ':' + req.path;
    const now = Date.now();
    const bucket = rateMap.get(key) || { count: 0, start: now };

    if (now - bucket.start > windowMs) {
      bucket.count = 0;
      bucket.start = now;
    }

    bucket.count++;
    rateMap.set(key, bucket);

    if (bucket.count > max) {
      return res.status(429).json({ error: 'Too many requests' });
    }

    next();
  };
}

module.exports = { rateLimit };

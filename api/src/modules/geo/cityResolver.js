// Simple nearest-city resolver using fixed coords for KZ cities
const cities = [
  { name: 'Almaty', lat: 43.238949, lng: 76.889709 },
  { name: 'Astana', lat: 51.160523, lng: 71.470356 },
  { name: 'Shymkent', lat: 42.3417, lng: 69.5901 },
  { name: 'Karaganda', lat: 49.806, lng: 73.085 },
  { name: 'Aktobe', lat: 50.2839, lng: 57.166 },
  { name: 'Taraz', lat: 42.9, lng: 71.3667 },
  { name: 'Pavlodar', lat: 52.2871, lng: 76.9674 },
  { name: 'Ust-Kamenogorsk', lat: 49.9481, lng: 82.6275 },
  { name: 'Semey', lat: 50.4111, lng: 80.2275 },
  { name: 'Atyrau', lat: 47.0945, lng: 51.9236 },
  { name: 'Kostanay', lat: 53.2144, lng: 63.6246 },
  { name: 'Kyzylorda', lat: 44.8488, lng: 65.4823 },
  { name: 'Uralsk', lat: 51.2278, lng: 51.3865 },
  { name: 'Petropavl', lat: 54.8728, lng: 69.143 },
  { name: 'Aktau', lat: 43.6532, lng: 51.1975 },
  { name: 'Temirtau', lat: 50.065, lng: 72.964 },
  { name: 'Turkistan', lat: 43.2973, lng: 68.2518 },
  { name: 'Kokshetau', lat: 53.2833, lng: 69.3833 },
  { name: 'Taldykorgan', lat: 45.0167, lng: 78.3667 },
  { name: 'Ekibastuz', lat: 51.7295, lng: 75.3229 }
];

function haversine(lat1, lon1, lat2, lon2) {
  const R = 6371;
  const toRad = d => (d * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a = Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) *
    Math.sin(dLon / 2) ** 2;
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

function resolveCity(lat, lng) {
  let best = null;
  let bestDist = Infinity;
  for (const c of cities) {
    const d = haversine(lat, lng, c.lat, c.lng);
    if (d < bestDist) {
      bestDist = d;
      best = c.name;
    }
  }
  return best || 'Almaty';
}

module.exports = { resolveCity };

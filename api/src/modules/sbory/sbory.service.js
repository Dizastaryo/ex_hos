const prisma = require('../../config/prisma');

async function createSbor(userId, data) {
  // create chat first
  const chat = await prisma.chat.create({
    data: {
      title: data.title,
      isGroup: true
    }
  });

  // creator is member
  await prisma.chatMember.create({ data: { chatId: chat.id, userId } });

  const sbor = await prisma.sbor.create({
    data: {
      title: data.title,
      city: data.city || 'Almaty',
      creatorId: userId,
      chatId: chat.id,
      lat: data.lat || null,
      lng: data.lng || null
    }
  });

  // add creator to sbor members
  await prisma.sborMember.create({ data: { sborId: sbor.id, userId } });

  return sbor;
}

async function joinSbor(userId, sborId) {
  const sbor = await prisma.sbor.findUnique({ where: { id: sborId } });
  if (!sbor) return null;

  await prisma.sborMember.create({ data: { sborId, userId } }).catch(() => {});
  await prisma.chatMember.create({ data: { chatId: sbor.chatId, userId } }).catch(() => {});

  return { success: true };
}

async function leaveSbor(userId, sborId) {
  const sbor = await prisma.sbor.findUnique({ where: { id: sborId } });
  if (!sbor) return null;

  await prisma.sborMember.deleteMany({ where: { sborId, userId } });
  await prisma.chatMember.deleteMany({ where: { chatId: sbor.chatId, userId } });

  return { success: true };
}

async function listSbory(city) {
  // support both sbor/sbory naming via try/catch
  try {
    return await prisma.sbor.findMany({
      where: { city, status: 'active' },
      orderBy: { createdAt: 'desc' }
    });
  } catch (e) {
    return prisma.sbory.findMany({
      where: { city, status: 'active' },
      orderBy: { createdAt: 'desc' }
    });
  }
}

function haversine(lat1, lon1, lat2, lon2) {
  const R = 6371; // km
  const toRad = d => (d * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) *
    Math.sin(dLon / 2) * Math.sin(dLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

async function nearbySbory({ lat, lng, radiusKm = 10 }) {
  const all = await prisma.sbory.findMany({
    where: { status: 'active', lat: { not: null }, lng: { not: null } }
  }).catch(() => prisma.sbor.findMany({
    where: { status: 'active', lat: { not: null }, lng: { not: null } }
  }));

  const filtered = all.filter(s => {
    const d = haversine(lat, lng, s.lat, s.lng);
    return d <= radiusKm;
  });

  // sort by distance
  filtered.sort((a, b) => {
    const da = haversine(lat, lng, a.lat, a.lng);
    const db = haversine(lat, lng, b.lat, b.lng);
    return da - db;
  });

  return filtered;
}

async function finishSbor(sborId, userId) {
  try {
    const s = await prisma.sbor.findUnique({ where: { id: sborId } });
    if (!s || s.creatorId !== userId) return null;
    return prisma.sbor.update({ where: { id: sborId }, data: { status: 'finished' } });
  } catch (e) {
    const s = await prisma.sbory.findUnique({ where: { id: sborId } });
    if (!s || s.creatorId !== userId) return null;
    return prisma.sbory.update({ where: { id: sborId }, data: { status: 'finished' } });
  }
}

module.exports = { createSbor, joinSbor, leaveSbor, listSbory, finishSbor, nearbySbory };

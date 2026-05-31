const prisma = require('../../config/prisma');

async function saveMedia(userId, url, type) {
  return prisma.media.create({
    data: {
      userId,
      url,
      type: type || null
    }
  });
}

module.exports = { saveMedia };

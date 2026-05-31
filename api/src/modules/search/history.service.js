const prisma = require('../../config/prisma');

async function addHistory(userId, query) {
  if (!query) return;
  try {
    await prisma.searchHistory.create({ data: { userId, query } });
  } catch (e) {}
}

async function getHistory(userId) {
  try {
    return prisma.searchHistory.findMany({
      where: { userId },
      orderBy: { createdAt: 'desc' },
      take: 10
    });
  } catch (e) {
    return [];
  }
}

module.exports = { addHistory, getHistory };

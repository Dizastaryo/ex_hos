const prisma = require('../../config/prisma');

async function searchAll(q) {
  const query = (q || '').toLowerCase();
  if (!query) return { users: [], posts: [] };

  const users = await prisma.user.findMany({
    where: {
      OR: [
        { username: { contains: query, mode: 'insensitive' } },
        { name: { contains: query, mode: 'insensitive' } }
      ]
    },
    take: 10
  });

  const posts = await prisma.post.findMany({
    where: { text: { contains: query, mode: 'insensitive' } },
    include: { author: true, _count: { select: { likes: true } } },
    take: 10
  });

  return { users, posts };
}

module.exports = { searchAll };

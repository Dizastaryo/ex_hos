const prisma = require('../../config/prisma');

async function getUserByUsername(username) {
  return prisma.user.findUnique({ where: { username } });
}

async function getUserPosts(username) {
  const user = await prisma.user.findUnique({ where: { username } });
  if (!user) return null;

  const posts = await prisma.post.findMany({
    where: { authorId: user.id },
    include: {
      author: true,
      likes: true,
      _count: { select: { likes: true } }
    },
    orderBy: { createdAt: 'desc' }
  });

  return { user, posts };
}

module.exports = { getUserByUsername, getUserPosts };

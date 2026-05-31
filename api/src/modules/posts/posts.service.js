const prisma = require('../../config/prisma');

async function getFeed(userId, cursor) {
  // 1) fetch following ids
  const following = await prisma.follow.findMany({
    where: { followerId: userId },
    select: { followingId: true }
  });

  const followingIds = following.map(f => f.followingId);

  // 2) primary feed: posts from following (and self)
  let posts = await prisma.post.findMany({
    where: {
      authorId: { in: [userId, ...followingIds] }
    },
    include: {
      author: true,
      likes: true,
      _count: { select: { likes: true } }
    },
    orderBy: { createdAt: 'desc' },
    take: 20,
    ...(cursor
      ? {
          skip: 1,
          cursor: { id: cursor }
        }
      : {})
  });

  // 3) fallback explore if empty
  if (posts.length === 0) {
    posts = await prisma.post.findMany({
      include: {
        author: true,
        likes: true,
        _count: { select: { likes: true } }
      },
      orderBy: { createdAt: 'desc' },
      take: 20
    });
  }

  return posts;
}

async function createPost(userId, data) {
  return prisma.post.create({
    data: {
      text: data.text || '',
      authorId: userId
    },
    include: { author: true }
  });
}

async function likePost(userId, postId) {
  // naive toggle: create like if not exists, else delete
  const existing = await prisma.postLike.findFirst({
    where: { userId, postId }
  });

  if (existing) {
    await prisma.postLike.delete({ where: { id: existing.id } });
    return { liked: false };
  }

  await prisma.postLike.create({ data: { userId, postId } });
  return { liked: true };
}

module.exports = { getFeed, createPost, likePost };

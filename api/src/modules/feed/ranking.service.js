const prisma = require('../../config/prisma');

// Personalized score: recency + likes + affinity
function score(post, affinity = 0) {
  const ageHours = (Date.now() - new Date(post.createdAt).getTime()) / 36e5;
  const likes = post._count?.likes || 0;
  return likes * 2 - ageHours * 0.5 + affinity * 3;
}

async function buildAffinity(userId) {
  // users whose posts current user liked
  const liked = await prisma.postLike.findMany({
    where: { userId },
    include: { post: true }
  });

  const map = new Map();

  for (const l of liked) {
    const authorId = l.post.authorId;
    map.set(authorId, (map.get(authorId) || 0) + 1);
  }

  return map;
}

async function getExplore(userId, cursor) {
  const posts = await prisma.post.findMany({
    include: {
      author: true,
      likes: true,
      _count: { select: { likes: true } }
    },
    orderBy: { createdAt: 'desc' },
    take: 100
  });

  const affinityMap = await buildAffinity(userId);

  const ranked = posts
    .map(p => {
      const affinity = affinityMap.get(p.authorId) || 0;
      return { p, s: score(p, affinity) };
    })
    .sort((a, b) => b.s - a.s)
    .map(x => x.p);

  const page = ranked.slice(0, 20);
  const nextCursor = page.length ? page[page.length - 1].id : null;

  return { items: page, nextCursor };
}

module.exports = { getExplore };

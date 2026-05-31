const prisma = require('../../config/prisma');

async function createStory(userId, data) {
  return prisma.story.create({
    data: {
      authorId: userId,
      mediaUrl: data.mediaUrl || null,
      text: data.text || null,
      expiresAt: new Date(Date.now() + 24 * 60 * 60 * 1000)
    },
    include: { author: true }
  });
}

async function getStoryFeed(userId) {
  const following = await prisma.follow.findMany({
    where: { followerId: userId },
    select: { followingId: true }
  });

  const ids = following.map(f => f.followingId);

  return prisma.story.findMany({
    where: {
      authorId: { in: [userId, ...ids] },
      expiresAt: { gt: new Date() }
    },
    include: { author: true },
    orderBy: { createdAt: 'desc' }
  });
}

async function viewStory(userId, storyId) {
  await prisma.storyView.upsert({
    where: {
      userId_storyId: { userId, storyId }
    },
    update: {},
    create: { userId, storyId }
  });
}

module.exports = { createStory, getStoryFeed, viewStory };

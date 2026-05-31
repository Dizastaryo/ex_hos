const prisma = require('../../config/prisma');

async function trackEvent({ userId, type, postId, storyId, metadata }) {
  return prisma.event.create({
    data: {
      userId,
      type,
      postId: postId || null,
      storyId: storyId || null,
      metadata: metadata ? JSON.stringify(metadata) : null
    }
  });
}

module.exports = { trackEvent };

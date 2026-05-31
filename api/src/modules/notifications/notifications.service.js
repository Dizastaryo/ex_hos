const prisma = require('../../config/prisma');
const { sendToUser } = require('../../realtime/ws');

async function createNotification({ type, userId, actorId, postId, commentId }) {
  // userId = recipient
  const n = await prisma.notification.create({
    data: {
      type,
      userId,
      actorId,
      postId: postId || null,
      commentId: commentId || null
    }
  });
  // realtime push
  sendToUser(userId, {
    type: 'notification',
    payload: {
      id: n.id,
      type: n.type,
      createdAt: n.createdAt,
      read: false,
      postId: n.postId || null,
      commentId: n.commentId || null
    }
  });
  return n;
}

async function getNotifications(userId) {
  return prisma.notification.findMany({
    where: { userId },
    include: {
      actor: true
    },
    orderBy: { createdAt: 'desc' },
    take: 50
  });
}

async function markAllRead(userId) {
  await prisma.notification.updateMany({
    where: { userId, readAt: null },
    data: { readAt: new Date() }
  });
}

module.exports = { createNotification, getNotifications, markAllRead };

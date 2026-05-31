const prisma = require('../../config/prisma');

async function getComments(postId) {
  return prisma.comment.findMany({
    where: { postId },
    include: {
      author: true,
      likes: true,
      _count: { select: { likes: true } }
    },
    orderBy: { createdAt: 'asc' }
  });
}

async function createComment(userId, postId, text, replyToId) {
  return prisma.comment.create({
    data: {
      text,
      postId,
      authorId: userId,
      replyToId: replyToId || null
    },
    include: { author: true }
  });
}

async function likeComment(userId, commentId) {
  const existing = await prisma.commentLike.findFirst({
    where: { userId, commentId }
  });

  if (existing) {
    await prisma.commentLike.delete({ where: { id: existing.id } });
    return { liked: false };
  }

  await prisma.commentLike.create({ data: { userId, commentId } });
  return { liked: true };
}

module.exports = { getComments, createComment, likeComment };

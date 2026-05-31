function avatar(u) {
  return u?.avatarUrl || u?.avatar_url || null;
}

function likesCount(entity) {
  if (!entity) return 0;
  if (typeof entity.likesCount === 'number') return entity.likesCount;
  if (entity._count && typeof entity._count.likes === 'number') return entity._count.likes;
  if (Array.isArray(entity.likes)) return entity.likes.length;
  if (typeof entity.likes === 'number') return entity.likes;
  return 0;
}

function isLiked(entity, userId) {
  if (!entity || !userId) return false;
  if (typeof entity.liked === 'boolean') return entity.liked;
  if (Array.isArray(entity.likes)) return entity.likes.some(l => l.userId === userId);
  return false;
}

module.exports = { avatar, likesCount, isLiked };

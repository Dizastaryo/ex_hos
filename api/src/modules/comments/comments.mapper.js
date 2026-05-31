const { avatar, likesCount, isLiked } = require('../../common/utils/normalize');

function mapComment(c, currentUserId) {
  return {
    id: c.id,
    text: c.text || '',
    createdAt: c.createdAt,
    author: {
      id: c.author.id,
      username: c.author.username,
      name: c.author.name,
      avatarUrl: avatar(c.author)
    },
    likesCount: likesCount(c),
    liked: isLiked(c, currentUserId),
    replyToId: c.replyToId || null
  };
}

module.exports = { mapComment };

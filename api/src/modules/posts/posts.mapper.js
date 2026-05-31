// Maps DB Post -> API DTO expected by Flutter
const { avatar, likesCount, isLiked } = require('../../common/utils/normalize');

function mapPost(post, currentUserId) {
  return {
    id: post.id,
    text: post.text || '',
    createdAt: post.createdAt,
    author: {
      id: post.author.id,
      username: post.author.username,
      name: post.author.name,
      avatarUrl: avatar(post.author)
    },
    likesCount: likesCount(post),
    liked: isLiked(post, currentUserId),
    mediaUrl: post.mediaUrl || post.media_url || null
  };
}

module.exports = { mapPost };

const { avatar } = require('../../common/utils/normalize');

function mapChat(c) {
  return {
    id: c.id,
    title: c.title,
    lastMessage: c.lastMessage
      ? {
          id: c.lastMessage.id,
          text: c.lastMessage.text,
          createdAt: c.lastMessage.createdAt
        }
      : null,
    unreadCount: c.unreadCount || 0,
    members: (c.members || []).map(m => ({
      id: m.user.id,
      username: m.user.username,
      name: m.user.name,
      avatarUrl: avatar(m.user)
    }))
  };
}

module.exports = { mapChat };

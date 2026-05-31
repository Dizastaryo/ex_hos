const { avatar } = require('../../common/utils/normalize');

function mapMessage(m) {
  return {
    id: m.id,
    text: m.text,
    createdAt: m.createdAt,
    sender: {
      id: m.sender.id,
      username: m.sender.username,
      name: m.sender.name,
      avatarUrl: avatar(m.sender)
    }
  };
}

module.exports = { mapMessage };

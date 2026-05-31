function mapStory(s) {
  return {
    id: s.id,
    mediaUrl: s.mediaUrl || s.media_url || null,
    text: s.text || null,
    createdAt: s.createdAt,
    expiresAt: s.expiresAt,
    author: {
      id: s.author.id,
      username: s.author.username,
      name: s.author.name,
      avatarUrl: s.author.avatarUrl || s.author.avatar_url || null
    }
  };
}

module.exports = { mapStory };

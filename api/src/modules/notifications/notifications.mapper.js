function mapNotification(n) {
  return {
    id: n.id,
    type: n.type,
    createdAt: n.createdAt,
    read: Boolean(n.readAt),
    actor: n.actor
      ? {
          id: n.actor.id,
          username: n.actor.username,
          name: n.actor.name,
          avatarUrl: n.actor.avatarUrl || n.actor.avatar_url || null
        }
      : null,
    postId: n.postId || null,
    commentId: n.commentId || null,
    // enrich placeholders for frontend expectations
    post: n.postId ? { id: n.postId } : null,
    comment: n.commentId ? { id: n.commentId } : null
  };
}

module.exports = { mapNotification };

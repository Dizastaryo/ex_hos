function mapUser(u) {
  return {
    id: u.id,
    username: u.username,
    name: u.name,
    avatarUrl: u.avatarUrl || u.avatar_url || null
  };
}

module.exports = { mapUser };

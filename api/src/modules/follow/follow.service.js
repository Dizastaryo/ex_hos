const prisma = require('../../config/prisma');

async function toggleFollow(currentUserId, targetUsername) {
  const target = await prisma.user.findUnique({ where: { username: targetUsername } });
  if (!target) return null;

  if (target.id === currentUserId) return { error: 'cannot_follow_self' };

  const existing = await prisma.follow.findFirst({
    where: { followerId: currentUserId, followingId: target.id }
  });

  if (existing) {
    await prisma.follow.delete({ where: { id: existing.id } });
    return { following: false };
  }

  await prisma.follow.create({
    data: { followerId: currentUserId, followingId: target.id }
  });

  return { following: true };
}

async function getFollowers(username) {
  const user = await prisma.user.findUnique({ where: { username } });
  if (!user) return null;

  const followers = await prisma.follow.findMany({
    where: { followingId: user.id },
    include: { follower: true }
  });

  return followers.map(f => f.follower);
}

async function getFollowing(username) {
  const user = await prisma.user.findUnique({ where: { username } });
  if (!user) return null;

  const following = await prisma.follow.findMany({
    where: { followerId: user.id },
    include: { following: true }
  });

  return following.map(f => f.following);
}

module.exports = { toggleFollow, getFollowers, getFollowing };

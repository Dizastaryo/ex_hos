require('dotenv').config({ path: '../../.env.local' });

const express = require('express');
const cors = require('cors');
const jwt = require('jsonwebtoken');

const prisma = require('./config/prisma');
const { getFeed, createPost, likePost } = require('./modules/posts/posts.service');
const { mapPost } = require('./modules/posts/posts.mapper');
const { getComments, createComment, likeComment } = require('./modules/comments/comments.service');
const { mapComment } = require('./modules/comments/comments.mapper');
const { getUserByUsername, getUserPosts } = require('./modules/users/users.service');
const { mapUser } = require('./modules/users/users.mapper');
const { toggleFollow, getFollowers, getFollowing } = require('./modules/follow/follow.service');
const { createStory, getStoryFeed, viewStory } = require('./modules/stories/stories.service');
const { mapStory } = require('./modules/stories/stories.mapper');
const { getChats, getMessages, sendMessage, markChatRead, getUnreadCount } = require('./modules/chat/chat.service');
const { mapMessage } = require('./modules/chat/chat.mapper');
const { mapChat } = require('./modules/chat/chat.chatMapper');
const { saveMedia } = require('./modules/media/media.service');
const { getExplore } = require('./modules/feed/ranking.service');
const { createSbor, joinSbor, leaveSbor, listSbory, finishSbor, nearbySbory } = require('./modules/sbory/sbory.service');
const cities = require('./modules/geo/cities');
const { resolveCity } = require('./modules/geo/cityResolver');

const http = require('http');
const { attachWSServer, sendToUser } = require('./realtime/ws');

const app = express();
app.use(cors());
app.use(express.json());
const { rateLimit } = require('./common/middleware/rateLimit');
app.use(rateLimit({ windowMs: 60000, max: 120 }));

// --- Auth middleware ---
function auth(req, res, next) {
  const header = req.headers.authorization;
  if (!header) return res.status(401).json({ error: 'No token' });

  const token = header.split(' ')[1];
  try {
    const payload = jwt.verify(token, process.env.JWT_ACCESS_SECRET);
    req.user = payload;
    next();
  } catch (e) {
    return res.status(401).json({ error: 'Invalid token' });
  }
}

// --- Auth ---
app.post('/api/v1/auth/verify-otp', async (req, res) => {
  // dev shortcut: always login first user
  const user = await prisma.user.findFirst();

  const accessToken = jwt.sign(
    { userId: user.id },
    process.env.JWT_ACCESS_SECRET,
    { expiresIn: '24h' }
  );

  res.json({ accessToken });
});

// --- Users ---
app.get('/api/v1/users/me', auth, async (req, res) => {
  const user = await prisma.user.findUnique({
    where: { id: req.user.userId }
  });

  res.json(user);
});

// --- Feed ---
app.get('/api/v1/feed', auth, async (req, res) => {
  const { cursor } = req.query;
  const posts = await getFeed(req.user.userId, cursor);
  const mapped = posts.map(p => mapPost(p, req.user.userId));

  const nextCursor = posts.length ? posts[posts.length - 1].id : null;

  res.json({ items: mapped, nextCursor });
});

// --- Explore (ranking) ---
app.get('/api/v1/explore', auth, async (req, res) => {
  const { cursor } = req.query;
  const { items, nextCursor } = await getExplore(req.user.userId, cursor);
  const mapped = items.map(p => mapPost(p, req.user.userId));
  res.json({ items: mapped, nextCursor });
});

// --- Cities ---
app.get('/api/v1/cities', auth, async (req, res) => {
  res.json({ items: cities });
});

// --- Sbory ---
app.get('/api/v1/sbory', auth, async (req, res) => {
  // city priority: query > user.preferred_city > default Almaty
  let city = req.query.city;
  if (!city) {
    const me = await prisma.user.findUnique({ where: { id: req.user.userId } });
    city = me?.preferredCity || me?.preferred_city || 'Almaty';
  }
  const items = await listSbory(city);
  res.json({ items });
});

app.post('/api/v1/sbory', auth, async (req, res) => {
  const sbor = await createSbor(req.user.userId, req.body);
  res.json(sbor);
});

app.post('/api/v1/sbory/:id/join', auth, async (req, res) => {
  const r = await joinSbor(req.user.userId, req.params.id);
  if (!r) return res.status(404).json({ error: 'Not found' });
  res.json(r);
});

app.post('/api/v1/sbory/:id/leave', auth, async (req, res) => {
  const r = await leaveSbor(req.user.userId, req.params.id);
  if (!r) return res.status(404).json({ error: 'Not found' });
  res.json(r);
});

// finish sbor
app.post('/api/v1/sbory/:id/finish', auth, async (req, res) => {
  const r = await finishSbor(req.params.id, req.user.userId);
  if (!r) return res.status(403).json({ error: 'Forbidden or not found' });
  res.json({ success: true });
});

// nearby sbory
app.get('/api/v1/sbory/nearby', auth, async (req, res) => {
  const { lat, lng, radius } = req.query;
  if (!lat || !lng) return res.status(400).json({ error: 'lat/lng required' });
  const items = await nearbySbory({ lat: Number(lat), lng: Number(lng), radiusKm: Number(radius) || 10 });
  res.json({ items });
});

// save preferred city
app.post('/api/v1/users/me/city', auth, async (req, res) => {
  const { city } = req.body;
  await prisma.user.update({ where: { id: req.user.userId }, data: { preferredCity: city } }).catch(async () => {
    await prisma.user.update({ where: { id: req.user.userId }, data: { preferred_city: city } });
  });
  res.json({ success: true });
});

// auto-detect city from lat/lng and save as preferred
app.post('/api/v1/users/me/city/auto', auth, async (req, res) => {
  const { lat, lng } = req.body;
  if (lat == null || lng == null) return res.status(400).json({ error: 'lat/lng required' });
  const city = resolveCity(Number(lat), Number(lng));
  await prisma.user.update({ where: { id: req.user.userId }, data: { preferredCity: city } }).catch(async () => {
    await prisma.user.update({ where: { id: req.user.userId }, data: { preferred_city: city } });
  });
  res.json({ city });
});

// --- Posts ---
app.post('/api/v1/posts', auth, async (req, res) => {
  const post = await createPost(req.user.userId, req.body);
  res.json(mapPost(post, req.user.userId));
});

// --- Post detail ---
app.get('/api/v1/posts/:id', auth, async (req, res) => {
  const post = await prisma.post.findUnique({
    where: { id: req.params.id },
    include: {
      author: true,
      likes: true,
      _count: { select: { likes: true } }
    }
  });

  if (!post) return res.status(404).json({ error: 'Not found' });

  res.json(mapPost(post, req.user.userId));
});

app.post('/api/v1/posts/:id/like', auth, async (req, res) => {
  await likePost(req.user.userId, req.params.id);
  const post = await prisma.post.findUnique({
    where: { id: req.params.id },
    include: {
      author: true,
      likes: true,
      _count: { select: { likes: true } }
    }
  });
  res.json(mapPost(post, req.user.userId));
});

// --- Users profile ---
app.get('/api/v1/users/:username', auth, async (req, res) => {
  const user = await getUserByUsername(req.params.username);
  if (!user) return res.status(404).json({ error: 'User not found' });
  res.json(mapUser(user));
});

app.get('/api/v1/users/:username/posts', auth, async (req, res) => {
  const result = await getUserPosts(req.params.username);
  if (!result) return res.status(404).json({ error: 'User not found' });

  const mapped = result.posts.map(p => mapPost(p, req.user.userId));
  res.json({ items: mapped });
});

// --- Media upload (improved placeholder) ---
const multer = require('multer');
const upload = multer({ dest: 'uploads/' });

app.post('/api/v1/media/upload', auth, upload.single('file'), async (req, res) => {
  // TODO: replace with S3 or proper storage later
  const base = process.env.CDN_BASE_URL || `${req.protocol}://${req.get('host')}`;
  const url = `/uploads/${req.file.filename}`;
  const absolute = `${base}${url}`;
  const media = await saveMedia(req.user.userId, absolute, req.file.mimetype);
  res.json({ url: absolute, id: media.id });
});

// --- Follow system ---
app.post('/api/v1/users/:username/follow', auth, async (req, res) => {
  const result = await toggleFollow(req.user.userId, req.params.username);
  if (!result) return res.status(404).json({ error: 'User not found' });
  if (result.error) return res.status(400).json({ error: result.error });
  res.json(result);
});

app.get('/api/v1/users/:username/followers', auth, async (req, res) => {
  const users = await getFollowers(req.params.username);
  if (!users) return res.status(404).json({ error: 'User not found' });
  res.json({ items: users.map(mapUser) });
});

app.get('/api/v1/users/:username/following', auth, async (req, res) => {
  const users = await getFollowing(req.params.username);
  if (!users) return res.status(404).json({ error: 'User not found' });
  res.json({ items: users.map(mapUser) });
});

// --- Comments ---
app.get('/api/v1/posts/:id/comments', auth, async (req, res) => {
  const comments = await getComments(req.params.id);
  const mapped = comments.map(c => mapComment(c, req.user.userId));
  res.json({ items: mapped });
});

app.post('/api/v1/posts/:id/comments', auth, async (req, res) => {
  const { text, replyToId } = req.body;
  const comment = await createComment(
    req.user.userId,
    req.params.id,
    text,
    replyToId
  );
  res.json(mapComment(comment, req.user.userId));
});

app.post('/api/v1/comments/:id/like', auth, async (req, res) => {
  await likeComment(req.user.userId, req.params.id);
  const comment = await prisma.comment.findUnique({
    where: { id: req.params.id },
    include: {
      author: true,
      likes: true,
      _count: { select: { likes: true } }
    }
  });
  res.json(mapComment(comment, req.user.userId));
});

// --- Stories ---
app.post('/api/v1/stories', auth, async (req, res) => {
  const story = await createStory(req.user.userId, req.body);
  res.json(mapStory(story));
});

app.get('/api/v1/stories/feed', auth, async (req, res) => {
  const stories = await getStoryFeed(req.user.userId);
  res.json({ items: stories.map(mapStory) });
});

app.post('/api/v1/stories/:id/view', auth, async (req, res) => {
  await viewStory(req.user.userId, req.params.id);
  res.json({ success: true });
});

// --- Chat ---
app.get('/api/v1/chats', auth, async (req, res) => {
  const chats = await getChats(req.user.userId);
  res.json({ items: chats.map(mapChat) });
});

app.get('/api/v1/chats/:id/messages', auth, async (req, res) => {
  const msgs = await getMessages(req.params.id);
  res.json({ items: msgs.map(mapMessage) });
});

app.post('/api/v1/chats/:id/messages', auth, async (req, res) => {
  const msg = await sendMessage(req.user.userId, req.params.id, req.body.text);
  res.json(mapMessage(msg));
});

// mark chat as read
app.post('/api/v1/chats/:id/read', auth, async (req, res) => {
  const r = await markChatRead(req.user.userId, req.params.id);
  res.json(r);
});

// get unread count for a chat
app.get('/api/v1/chats/:id/unread', auth, async (req, res) => {
  const count = await getUnreadCount(req.user.userId, req.params.id);
  res.json({ unreadCount: count });
});

// --- Start (HTTP + WS) ---
const PORT = process.env.APP_PORT || 8001;
const server = http.createServer(app);
attachWSServer(server);

server.listen(PORT, () => {
  console.log('API+WS started on port', PORT);
});

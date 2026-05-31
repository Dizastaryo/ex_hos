const express = require('express');
const multer = require('multer');
const cors = require('cors');

const app = express();
app.use(cors());
app.use(express.json());

const upload = multer({ dest: 'uploads/' });

// --- In-memory data (temporary, replace with DB later) ---
let users = [
  { id: '1', username: 'test', name: 'Test User', avatarUrl: null }
];

let posts = [
  {
    id: '1',
    author: users[0],
    text: 'Первый пост',
    mediaUrl: null,
    likesCount: 0,
    liked: false,
    createdAt: new Date().toISOString()
  }
];

// --- Auth ---
app.post('/api/v1/auth/send-otp', (req, res) => {
  res.json({ success: true });
});

app.post('/api/v1/auth/verify-otp', (req, res) => {
  res.json({ accessToken: 'dev-token', refreshToken: 'dev-refresh' });
});

// --- Users ---
app.get('/api/v1/users/me', (req, res) => {
  res.json(users[0]);
});

// --- Feed ---
app.get('/api/v1/feed', (req, res) => {
  res.json({ items: posts });
});

// --- Posts ---
app.get('/api/v1/posts', (req, res) => {
  res.json(posts);
});

app.post('/api/v1/posts', (req, res) => {
  const p = {
    id: String(posts.length + 1),
    author: users[0],
    text: req.body.text || '',
    mediaUrl: req.body.mediaUrl || null,
    likesCount: 0,
    liked: false,
    createdAt: new Date().toISOString()
  };
  posts.unshift(p);
  res.json(p);
});

app.post('/api/v1/posts/:id/like', (req, res) => {
  const post = posts.find(p => p.id === req.params.id);
  if (!post) return res.status(404).end();
  post.liked = !post.liked;
  post.likesCount += post.liked ? 1 : -1;
  res.json(post);
});

// --- Media upload ---
app.post('/api/v1/media/upload', upload.single('file'), (req, res) => {
  res.json({ url: `/uploads/${req.file.filename}` });
});

// --- Start server ---
const PORT = 8001;
app.listen(PORT, () => {
  console.log(`API running on http://localhost:${PORT}`);
});

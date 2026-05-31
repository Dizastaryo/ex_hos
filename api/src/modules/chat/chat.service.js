const prisma = require('../../config/prisma');
const { sendToUser } = require('../../realtime/ws');

async function getChats(userId) {
  const chats = await prisma.chat.findMany({
    where: { members: { some: { userId } } },
    include: {
      members: { include: { user: true } },
      lastMessage: true
    },
    orderBy: { updatedAt: 'desc' }
  });
  // naive unread: total messages in chat (better than 0; replace with read receipts later)
  const withCounts = await Promise.all(
    chats.map(async c => {
      const count = await prisma.message.count({ where: { chatId: c.id } }).catch(() => 0);
      return { ...c, unreadCount: count };
    })
  );
  return withCounts;
}

async function getMessages(chatId) {
  const messages = await prisma.message.findMany({
    where: { chatId },
    include: { sender: true },
    orderBy: { createdAt: 'asc' }
  });
  return messages;
}

async function sendMessage(userId, chatId, text) {
  const msg = await prisma.message.create({
    data: {
      chatId,
      senderId: userId,
      text
    },
    include: { sender: true }
  });

  // push to all chat members
  const members = await prisma.chatMember.findMany({ where: { chatId } });
  for (const m of members) {
    sendToUser(m.userId, {
      type: 'chat_message',
      payload: {
        id: msg.id,
        chatId,
        text: msg.text,
        createdAt: msg.createdAt,
        sender: {
          id: msg.sender.id,
          username: msg.sender.username
        }
      }
    });
  }

  return msg;
}

async function markChatRead(userId, chatId) {
  // mark all messages in chat as read for user
  const messages = await prisma.message.findMany({ where: { chatId }, select: { id: true } });
  const ops = messages.map(m =>
    prisma.messageRead.upsert({
      where: { userId_messageId: { userId, messageId: m.id } },
      update: {},
      create: { userId, messageId: m.id }
    }).catch(() => null)
  );
  await Promise.all(ops);
  return { success: true };
}

async function getUnreadCount(userId, chatId) {
  const total = await prisma.message.count({ where: { chatId } }).catch(() => 0);
  const read = await prisma.messageRead.count({ where: { userId, message: { chatId } } }).catch(() => 0);
  return Math.max(total - read, 0);
}

module.exports = { getChats, getMessages, sendMessage, markChatRead, getUnreadCount };

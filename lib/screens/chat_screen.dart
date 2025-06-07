import 'dart:io';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import '../services/chat_service.dart';

class ChatPage extends StatefulWidget {
  final ChatService chatService;

  const ChatPage({super.key, required this.chatService});

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends State<ChatPage> {
  final List<Map<String, dynamic>> _messages = [];
  final TextEditingController _controller = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  bool _isLoading = false;

  @override
  void initState() {
    super.initState();
    _loadHistory();
  }

  Future<void> _loadHistory() async {
    final history = await widget.chatService.getUserChatHistory();
    // Переименовываем поле message → text
    final formatted = history.map((msg) {
      return {
        'sender': msg['sender'],
        'text': msg['message'],
        'timestamp': msg['timestamp'],
      };
    }).toList();
    // При желании можно раскомментировать сортировку по времени:
    // formatted.sort((a, b) =>
    //   DateTime.parse(a['timestamp']).compareTo(DateTime.parse(b['timestamp'])));
    setState(() {
      _messages.clear();
      _messages.addAll(formatted);
    });
    _scrollToBottom();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent + 80,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  Future<void> _sendMessage(String text) async {
    if (text.trim().isEmpty) return;
    setState(() {
      _messages.add({'sender': 'user', 'text': text});
      _isLoading = true;
    });
    _controller.clear();
    _scrollToBottom();

    try {
      final response = await widget.chatService.sendMessage(text);
      final answer = response['diagnosis'] ?? 'Нет ответа';
      setState(() {
        _messages.add({'sender': 'model', 'text': answer});
      });
    } catch (e) {
      setState(() {
        _messages.add({'sender': 'model', 'text': 'Ошибка: $e'});
      });
    } finally {
      setState(() => _isLoading = false);
      _scrollToBottom();
    }
  }

  Future<void> _sendImage() async {
    final picker = ImagePicker();
    final picked = await picker.pickImage(source: ImageSource.gallery);
    if (picked == null) return;

    final file = File(picked.path);
    setState(() {
      _messages.add({'sender': 'user', 'text': '[Фото отправлено]'});
      _isLoading = true;
    });
    _scrollToBottom();

    try {
      final result = await widget.chatService.predictImage(file);
      setState(() {
        _messages.add({'sender': 'model', 'text': 'Диагноз по фото: $result'});
      });
    } catch (e) {
      setState(() {
        _messages.add({'sender': 'model', 'text': 'Ошибка при распознавании: $e'});
      });
    } finally {
      setState(() => _isLoading = false);
      _scrollToBottom();
    }
  }

  Widget _buildMessage(Map<String, dynamic> message) {
    final sender = message['sender'] as String;
    final rawText = message['text'] as String;

    // Выбираем выравнивание и префикс для разных отправителей
    final isUser = sender == 'user';
    final alignment = isUser ? Alignment.centerRight : Alignment.centerLeft;
    final color = isUser ? Colors.teal[200] : Colors.grey[300];
    final radius = BorderRadius.only(
      topLeft: const Radius.circular(12),
      topRight: const Radius.circular(12),
      bottomLeft: Radius.circular(isUser ? 12 : 0),
      bottomRight: Radius.circular(isUser ? 0 : 12),
    );

    String displayText;
    if (sender == 'model') {
      displayText = 'ИИ: $rawText';
    } else if (sender == 'moderator') {
      displayText = 'Доктор: $rawText';
    } else {
      displayText = rawText;
    }

    return Align(
      alignment: alignment,
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 6, horizontal: 12),
        padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 14),
        decoration: BoxDecoration(color: color, borderRadius: radius),
        child: Text(displayText, style: const TextStyle(fontSize: 16)),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Чат с диагностикой')),
      body: Column(
        children: [
          Expanded(
            child: ListView.builder(
              controller: _scrollController,
              padding: const EdgeInsets.symmetric(vertical: 10),
              itemCount: _messages.length,
              itemBuilder: (context, index) => _buildMessage(_messages[index]),
            ),
          ),
          if (_isLoading)
            const Padding(
              padding: EdgeInsets.all(8.0),
              child: CircularProgressIndicator(),
            ),
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              child: Row(
                children: [
                  IconButton(
                    icon: const Icon(Icons.image),
                    onPressed: _sendImage,
                  ),
                  Expanded(
                    child: TextField(
                      controller: _controller,
                      decoration: const InputDecoration(
                        hintText: 'Напишите сообщение...',
                        border: OutlineInputBorder(),
                        contentPadding: EdgeInsets.symmetric(horizontal: 12),
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.send),
                    onPressed: () => _sendMessage(_controller.text),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

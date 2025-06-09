// chat_page.dart
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
    final formatted = history
        .map((msg) => {
              'sender': msg['sender'],
              'text': msg['message'],
              'timestamp': msg['timestamp'],
            })
        .toList();

    setState(() {
      _messages
        ..clear()
        ..addAll(formatted);
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
        _messages.add({'sender': 'model', 'text': 'Ошибка: $e'});
      });
    } finally {
      setState(() => _isLoading = false);
      _scrollToBottom();
    }
  }

  Widget _buildMessage(Map<String, dynamic> message) {
    final sender = message['sender'] ?? '';
    final text = message['text'] ?? '';
    final isUser = sender == 'user';

    final align = isUser ? Alignment.centerRight : Alignment.centerLeft;
    final color = isUser
        ? const Color(0xFF30D5C8).withOpacity(0.2)
        : Colors.grey.shade200;

    String displayText = text;
    if (sender == 'model') displayText = 'ИИ: $text';
    if (sender == 'moderator') displayText = 'Доктор: $text';

    return Align(
      alignment: align,
      child: Card(
        color: color,
        margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        elevation: 2,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Text(displayText, style: const TextStyle(fontSize: 16)),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Консультация ИИ'),
        automaticallyImplyLeading: false,
        backgroundColor: const Color(0xFF30D5C8),
      ),
      body: Column(
        children: [
          Expanded(
            child: ListView.builder(
              controller: _scrollController,
              itemCount: _messages.length,
              itemBuilder: (context, index) => _buildMessage(_messages[index]),
            ),
          ),
          if (_isLoading)
            const Padding(
                padding: EdgeInsets.all(8), child: CircularProgressIndicator()),
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  IconButton(
                    icon: const Icon(Icons.image, color: Color(0xFF30D5C8)),
                    onPressed: _sendImage,
                  ),
                  Expanded(
                    child: TextField(
                      controller: _controller,
                      decoration: InputDecoration(
                        hintText: 'Напишите сообщение...',
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(20),
                        ),
                        filled: true,
                        fillColor: Colors.grey[100],
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.send, color: Color(0xFF30D5C8)),
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

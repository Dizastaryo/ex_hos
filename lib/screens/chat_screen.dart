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

  // Основной цвет
  final Color _primaryColor = const Color(0xFF30D5C8);
  final Color _userMessageColor = const Color(0xFF30D5C8);
  final Color _aiMessageColor = const Color(0xFFF5F7F9);

  @override
  void initState() {
    super.initState();
    _loadHistory();
  }

  Future<void> _loadHistory() async {
    final history = await widget.chatService.getUserChatHistory();
    final formatted = history.map((msg) {
      return {
        'sender': msg['sender'],
        'text': msg['message'],
        'timestamp': msg['timestamp'],
      };
    }).toList();
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
          _scrollController.position.maxScrollExtent + 100,
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
        _messages
            .add({'sender': 'model', 'text': 'Ошибка при распознавании: $e'});
      });
    } finally {
      setState(() => _isLoading = false);
      _scrollToBottom();
    }
  }

  Widget _buildMessage(Map<String, dynamic> message) {
    final sender = message['sender'] as String;
    final text = message['text'] as String;
    final isUser = sender == 'user';

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      child: Row(
        mainAxisAlignment:
            isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (!isUser) ...[
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: _primaryColor.withOpacity(0.2),
                borderRadius: BorderRadius.circular(16),
              ),
              child: Icon(Icons.auto_awesome, size: 18, color: _primaryColor),
            ),
            const SizedBox(width: 8),
          ],
          Flexible(
            child: Container(
              padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
              decoration: BoxDecoration(
                color: isUser ? _userMessageColor : _aiMessageColor,
                borderRadius: BorderRadius.only(
                  topLeft: const Radius.circular(18),
                  topRight: const Radius.circular(18),
                  bottomLeft: Radius.circular(isUser ? 18 : 4),
                  bottomRight: Radius.circular(isUser ? 4 : 18),
                ),
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withOpacity(0.05),
                    blurRadius: 6,
                    offset: const Offset(0, 2),
                  )
                ],
              ),
              child: Text(
                text,
                style: TextStyle(
                  fontSize: 15,
                  height: 1.4,
                  color: isUser ? Colors.white : Colors.grey[800],
                ),
              ),
            ),
          ),
          if (isUser) ...[
            const SizedBox(width: 8),
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: _primaryColor.withOpacity(0.1),
                borderRadius: BorderRadius.circular(16),
              ),
              child: Icon(Icons.person, size: 18, color: _primaryColor),
            ),
          ],
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Чат с диагностикой',
            style: TextStyle(fontWeight: FontWeight.w600)),
        centerTitle: true,
        backgroundColor: Colors.white,
        elevation: 2,
        automaticallyImplyLeading: false,
        shadowColor: Colors.black.withOpacity(0.1),
        iconTheme: IconThemeData(color: Colors.grey[800]),
      ),
      body: Column(
        children: [
          Expanded(
            child: Container(
              color: Colors.grey[50],
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: ListView.builder(
                controller: _scrollController,
                itemCount: _messages.length,
                itemBuilder: (context, index) =>
                    _buildMessage(_messages[index]),
              ),
            ),
          ),
          if (_isLoading)
            Padding(
              padding: const EdgeInsets.only(bottom: 16),
              child: CircularProgressIndicator(color: _primaryColor),
            ),
          Container(
            color: Colors.white,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            child: SafeArea(
              child: Row(
                children: [
                  Container(
                    decoration: BoxDecoration(
                      color: Colors.grey[100],
                      borderRadius: BorderRadius.circular(24),
                    ),
                    child: IconButton(
                      icon: Icon(Icons.image, color: Colors.grey[700]),
                      onPressed: _sendImage,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        color: Colors.grey[100],
                        borderRadius: BorderRadius.circular(24),
                        boxShadow: [
                          BoxShadow(
                            color: Colors.black.withOpacity(0.05),
                            blurRadius: 2,
                            offset: const Offset(0, 1),
                          )
                        ],
                      ),
                      child: TextField(
                        controller: _controller,
                        decoration: InputDecoration(
                          hintText: 'Напишите сообщение...',
                          hintStyle: TextStyle(color: Colors.grey[500]),
                          border: InputBorder.none,
                          contentPadding: const EdgeInsets.symmetric(
                              horizontal: 20, vertical: 16),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Container(
                    decoration: BoxDecoration(
                      color: _primaryColor,
                      borderRadius: BorderRadius.circular(24),
                    ),
                    child: IconButton(
                      icon: const Icon(Icons.send, color: Colors.white),
                      onPressed: () => _sendMessage(_controller.text),
                    ),
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

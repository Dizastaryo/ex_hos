// moderator_chat_page.dart
import 'package:flutter/material.dart';
import '../services/chat_service.dart';

class ModeratorChatPage extends StatefulWidget {
  final int userId;
  final String patientName;
  final ChatService chatService;

  const ModeratorChatPage({
    Key? key,
    required this.userId,
    required this.patientName,
    required this.chatService,
  }) : super(key: key);

  @override
  State<ModeratorChatPage> createState() => _ModeratorChatPageState();
}

class _ModeratorChatPageState extends State<ModeratorChatPage> {
  late final ChatService _chatService;
  final TextEditingController _controller = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  late Future<List<dynamic>> _chatFuture;
  final List<Map<String, dynamic>> _messages = [];
  bool _isLoading = false;

  @override
  void initState() {
    super.initState();
    _chatService = widget.chatService;
    _chatFuture = _chatService.getChatHistoryModerator(widget.userId);
  }

  Future<void> _sendMessage() async {
    final text = _controller.text.trim();
    if (text.isEmpty) return;

    setState(() {
      _messages.add({'sender': 'moderator', 'message': text});
      _isLoading = true;
    });
    _controller.clear();
    _scrollToBottom();

    try {
      await _chatService.sendModeratorReply(widget.userId, text);
      setState(() {
        _chatFuture = _chatService.getChatHistoryModerator(widget.userId);
      });
    } catch (e) {
      setState(() {
        _messages.add({'sender': 'system', 'message': 'Ошибка отправки: $e'});
      });
    } finally {
      setState(() => _isLoading = false);
      _scrollToBottom();
    }
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

  Widget _buildMessage(Map<String, dynamic> msg) {
    final sender = msg['sender'];
    final text = msg['message'] ?? '';
    final isModerator = sender == 'moderator';

    final color = isModerator
        ? const Color(0xFF30D5C8).withOpacity(0.2)
        : Colors.grey.shade200;
    final align = isModerator ? Alignment.centerRight : Alignment.centerLeft;

    String displayText = text;
    if (sender == 'user') {
      displayText = '${widget.patientName}: $text';
    } else if (sender == 'model') {
      displayText = 'ИИ: $text';
    }

    return Align(
      alignment: align,
      child: Card(
        margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        elevation: 2,
        color: color,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
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
        title: Text('Чат с ${widget.patientName}'),
        backgroundColor: const Color(0xFF30D5C8),
      ),
      body: Column(
        children: [
          Expanded(
            child: FutureBuilder<List<dynamic>>(
              future: _chatFuture,
              builder: (context, snapshot) {
                if (snapshot.connectionState == ConnectionState.waiting) {
                  return const Center(child: CircularProgressIndicator());
                }
                if (snapshot.hasError) {
                  return Center(child: Text('Ошибка: ${snapshot.error}'));
                }

                final history = snapshot.data ?? [];
                _messages
                  ..clear()
                  ..addAll(history.cast<Map<String, dynamic>>());

                return ListView.builder(
                  controller: _scrollController,
                  itemCount: _messages.length,
                  itemBuilder: (context, index) =>
                      _buildMessage(_messages[index]),
                );
              },
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
                  Expanded(
                    child: TextField(
                      controller: _controller,
                      decoration: InputDecoration(
                        hintText: 'Введите сообщение...',
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
                    onPressed: _sendMessage,
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

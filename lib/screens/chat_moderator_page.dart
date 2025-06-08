import 'package:flutter/material.dart';
import 'package:dio/dio.dart';
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
  bool _isLoading = false;

  @override
  void initState() {
    super.initState();
    _chatService = widget.chatService;
    _chatFuture = _chatService.getChatHistoryModerator(widget.userId);
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
      // После отправки обновляем историю
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

  // Хранилище уже загруженных сообщений для мгновенного отображения
  final List<Map<String, dynamic>> _messages = [];

  Widget _buildMessage(Map<String, dynamic> msg) {
    final sender = msg['sender'] as String;
    final rawText = msg['message'] as String;
    final isModerator = sender == 'moderator';
    final alignment =
        isModerator ? Alignment.centerRight : Alignment.centerLeft;
    final color = isModerator ? Colors.blue[100] : Colors.grey[300];
    final borderRadius = BorderRadius.only(
      topLeft: const Radius.circular(12),
      topRight: const Radius.circular(12),
      bottomLeft: Radius.circular(isModerator ? 12 : 0),
      bottomRight: Radius.circular(isModerator ? 0 : 12),
    );

    String displayText = rawText;
    if (sender == 'user') {
      // Вместо «User:» используем patientName
      displayText = '${widget.patientName}: $rawText';
    } else if (sender == 'model') {
      displayText = 'ИИ: $rawText';
    }

    return Align(
      alignment: alignment,
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 6, horizontal: 12),
        padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 14),
        decoration: BoxDecoration(
          color: color,
          borderRadius: borderRadius,
        ),
        child: Text(displayText, style: const TextStyle(fontSize: 16)),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('Чат с ${widget.patientName}')),
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

                // Получаем список сообщений из API
                final loaded = snapshot.data ?? [];
                // Синхронизируем с локальным _messages (для мгновенного добавления нового)
                _messages
                  ..clear()
                  ..addAll(loaded.map((e) => e as Map<String, dynamic>));

                return ListView.builder(
                  controller: _scrollController,
                  padding: const EdgeInsets.symmetric(vertical: 10),
                  itemCount: _messages.length,
                  itemBuilder: (context, index) {
                    return _buildMessage(_messages[index]);
                  },
                );
              },
            ),
          ),

          // Индикатор загрузки при отправке
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
                  Expanded(
                    child: TextField(
                      controller: _controller,
                      decoration: const InputDecoration(
                        hintText: 'Введите сообщение...',
                        border: OutlineInputBorder(),
                        contentPadding: EdgeInsets.symmetric(horizontal: 12),
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.send),
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

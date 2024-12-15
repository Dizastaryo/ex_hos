import 'package:flutter/material.dart';

// Модель уведомлений
class NotificationItem {
  final String title;
  final String description;
  final String payload;

  NotificationItem({
    required this.title,
    required this.description,
    required this.payload,
  });
}

class NotificationsScreen extends StatefulWidget {
  const NotificationsScreen({super.key});

  @override
  _NotificationsScreenState createState() => _NotificationsScreenState();
}

class _NotificationsScreenState extends State<NotificationsScreen> {
  final List<NotificationItem> notifications = [
    NotificationItem(
        title: 'Аренда заканчивается',
        description: 'Бокс #101',
        payload: '101'),
    NotificationItem(
        title: 'Новая аренда', description: 'Бокс #102', payload: '102'),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        backgroundColor: Colors.white,
        elevation: 0,
        centerTitle: true,
        title: const Text(
          'Уведомления',
          style: TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 18,
            color: Colors.black,
          ),
        ),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back, color: Colors.black),
          onPressed: () {
            Navigator.pop(context);
          },
        ),
      ),
      body: ListView.builder(
        padding: const EdgeInsets.all(16.0),
        itemCount: notifications.length,
        itemBuilder: (context, index) {
          final notification = notifications[index];
          return Card(
            margin: const EdgeInsets.only(bottom: 10),
            elevation: 2,
            child: ListTile(
              leading: const Icon(Icons.notifications, color: Colors.green),
              title: Text(notification.title),
              subtitle: Text(notification.description),
              onTap: () {
                _onNotificationTap(notification.payload);
              },
            ),
          );
        },
      ),
    );
  }

  void _onNotificationTap(String payload) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => NotificationDetailScreen(payload: payload),
      ),
    );
  }
}

class NotificationDetailScreen extends StatelessWidget {
  final String payload;

  const NotificationDetailScreen({super.key, required this.payload});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Детали уведомления'),
      ),
      body: Center(
        child: Text('Подробности для уведомления $payload'),
      ),
    );
  }
}

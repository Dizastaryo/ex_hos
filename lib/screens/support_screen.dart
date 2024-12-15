import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

class SupportScreen extends StatelessWidget {
  // Метод для открытия WhatsApp
  Future<void> _launchWhatsApp() async {
    final Uri url = Uri.parse(
        'https://wa.me/message/Z63SRSL26VKYL1'); // Ссылка для перехода на WhatsApp
    if (await canLaunchUrl(url)) {
      await launchUrl(url); // Открывает WhatsApp
    } else {
      throw 'Не удалось открыть ссылку $url';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Поддержка')),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Используем картинку для кнопки WhatsApp
            GestureDetector(
              onTap: _launchWhatsApp, // При нажатии открываем WhatsApp
              child: Image.asset('assets/images/what.png',
                  width: 100, height: 100),
            ),
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: _launchWhatsApp, // Нажав на кнопку, откроется WhatsApp
              child: const Text('Связаться через WhatsApp'),
            ),
          ],
        ),
      ),
    );
  }
}

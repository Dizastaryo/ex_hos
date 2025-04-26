import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

class SupportScreen extends StatelessWidget {
  Future<void> _launchWhatsApp() async {
    final Uri url = Uri.parse('https://wa.me/message/Z63SRSL26VKYL1');
    if (await canLaunchUrl(url)) {
      await launchUrl(url);
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
            GestureDetector(
              onTap: _launchWhatsApp,
              child: Image.asset('assets/images/what.png',
                  width: 100, height: 100),
            ),
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: _launchWhatsApp,
              child: const Text('Связаться через WhatsApp'),
            ),
          ],
        ),
      ),
    );
  }
}

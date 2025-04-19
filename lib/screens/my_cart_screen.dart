import 'package:flutter/material.dart';
import 'package:dio/dio.dart';

class MyCartScreen extends StatefulWidget {
  const MyCartScreen({super.key});

  @override
  State<MyCartScreen> createState() => _MyCartScreenState();
}

class _MyCartScreenState extends State<MyCartScreen> {
  String _output = 'Нажми кнопку, чтобы увидеть данные запроса';

  Future<void> _checkHeadersAndCookies() async {
    final dio = Dio();

    try {
      final response = await dio.get(
        'http://127.0.0.1:8000/products/', // замени на актуальный эндпоинт
        options: Options(
          headers: {
            // Пример ручной передачи токена, если нужен
            // 'Authorization': 'Bearer your_access_token',
          },
        ),
      );

      final buffer = StringBuffer();

      buffer.writeln('🔹 Отправленные заголовки:');
      dio.options.headers.forEach((key, value) {
        buffer.writeln('$key: $value');
      });

      buffer.writeln('\n🔸 Заголовки ответа:');
      response.headers.forEach((key, values) {
        buffer.writeln('$key: ${values.join('; ')}');
      });

      buffer.writeln('\n🍪 Set-Cookie из ответа:');
      final cookies = response.headers.map['set-cookie'];
      if (cookies != null) {
        for (var cookie in cookies) {
          buffer.writeln(cookie);
        }
      } else {
        buffer.writeln('Нет cookies в ответе.');
      }

      setState(() {
        _output = buffer.toString();
      });
    } catch (e) {
      setState(() {
        _output = '❌ Ошибка: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Данные запроса')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          children: [
            ElevatedButton(
              onPressed: _checkHeadersAndCookies,
              child: const Text('Показать данные запроса'),
            ),
            const SizedBox(height: 20),
            Expanded(
              child: SingleChildScrollView(
                child: Text(
                  _output,
                  style: const TextStyle(fontSize: 14),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

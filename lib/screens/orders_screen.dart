import 'package:flutter/material.dart';
import 'package:dio/dio.dart';
import 'package:provider/provider.dart';

class OrdersScreen extends StatefulWidget {
  const OrdersScreen({super.key});

  @override
  State<OrdersScreen> createState() => _OrdersScreenState();
}

class _OrdersScreenState extends State<OrdersScreen> {
  String _output = 'Нажми кнопку, чтобы увидеть данные запроса';

  Future<void> _checkHeadersAndCookies() async {
    // Получаем Dio из провайдера
    final dio = Provider.of<Dio>(context, listen: false);

    // Получаем токен из хранилища или контекста
    final token = await _getAccessToken();

    // Настроим Dio для отправки токена
    dio.options.headers['Authorization'] = 'Bearer $token';

    try {
      final response = await dio.get(
        'http://172.20.10.2:8000/products/', // замени на актуальный эндпоинт для заказов
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

  // Этот метод должен возвращать access токен (например, из SharedPreferences или другого хранилища)
  Future<String> _getAccessToken() async {
    // Здесь ты можешь получить токен из хранилища (например, SharedPreferences, локальное хранилище и т.п.)
    // Для демонстрации возвращаю пример токена.
    return 'your_jwt_access_token';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Заказы'),
      ),
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

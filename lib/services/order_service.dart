import 'package:dio/dio.dart';

class OrderService {
  static const _baseUrl =
      'http://172.20.10.2:8000'; // Убедитесь, что URL соответствует вашему FastAPI
  final Dio _dio;

  OrderService(this._dio);

  Future<Map<String, dynamic>> createOrder(
      List<Map<String, int>> items, String shippingAddress) async {
    final response = await _dio.post(
      '$_baseUrl/orders/',
      data: {
        'items': items,
        'shipping_address': shippingAddress,
      },
    );
    return response.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getOrderById(int orderId) async {
    try {
      final response = await _dio.get('$_baseUrl/orders/$orderId');
      return response.data as Map<String, dynamic>;
    } catch (e) {
      throw Exception('Не удалось найти заказ');
    }
  }

  Future<List<dynamic>> getMyOrders() async {
    final response = await _dio.get('$_baseUrl/orders/');
    return response.data as List<dynamic>;
  }

  Future<void> cancelOrder(int orderId) async {
    try {
      await _dio.post('$_baseUrl/orders/$orderId/cancel');
    } catch (e) {
      throw Exception('Не удалось отменить заказ');
    }
  }

  // Метод для создания платежа
  Future<Map<String, dynamic>> createPayment(int orderId, String method) async {
    final response = await _dio.post(
      '$_baseUrl/payments/create', // Отправка запроса на создание платежа
      data: {
        'order_id': orderId,
        'amount': 100.0
      }, // Тут можно отправить сумму как параметр
    );
    return response.data as Map<String, dynamic>;
  }

  // Метод для получения статуса платежа
  Future<Map<String, dynamic>> getPaymentStatus(int orderId) async {
    final response = await _dio
        .get('$_baseUrl/payments/status/$orderId'); // Запрос статуса платежа
    return response.data as Map<String, dynamic>;
  }

  String _formatError(DioException e) {
    return e.response != null
        ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
        : 'Сетевая ошибка: ${e.message}';
  }

  Future<List<dynamic>> getAllOrders() async {
    try {
      final response = await _dio.get('$_baseUrl/orders/all');
      return response.data as List<dynamic>;
    } catch (e) {
      throw Exception('Не удалось получить все заказы');
    }
  }
}

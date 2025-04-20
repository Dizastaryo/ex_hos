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
  Future<Map<String, dynamic>> createPayment(int orderId, double amount) async {
    final response = await _dio.post(
      '$_baseUrl/payments/create',
      data: {
        'order_id': orderId,
        'amount': amount,
      },
    );
    return response.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getPaymentStatus(int orderId) async {
    final response = await _dio.get('$_baseUrl/payments/status/$orderId');
    return response.data as Map<String, dynamic>;
  }

  String _formatError(DioException e) {
    return e.response != null
        ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
        : 'Сетевая ошибка: ${e.message}';
  }

  Future<Map<String, dynamic>> updateOrderStatus(
      int orderId, String newStatus) async {
    final response = await _dio.put(
      '$_baseUrl/admin/orders/$orderId/status',
      queryParameters: {'status': newStatus},
    );
    return response.data as Map<String, dynamic>;
  }

  /// Для админа/модератора — получить все заказы
  Future<List<dynamic>> getAllOrders() async {
    final response = await _dio.get('$_baseUrl/all');
    return response.data as List<dynamic>;
  }
}

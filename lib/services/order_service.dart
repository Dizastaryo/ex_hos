import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';

class OrderService {
  // Берём базовый URL из .env
  final String _baseUrl = dotenv.env['API_BASE_URL']!;

  final Dio _dio;

  OrderService(this._dio);

  Future<Map<String, dynamic>> createOrder(
      List<Map<String, int>> items, String shippingAddress) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/orders/',
        data: {
          'items': items,
          'shipping_address': shippingAddress,
        },
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<Map<String, dynamic>> getOrderById(int orderId) async {
    try {
      final response = await _dio.get('$_baseUrl/orders/$orderId');
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<List<dynamic>> getMyOrders() async {
    try {
      final response = await _dio.get('$_baseUrl/orders/');
      return response.data as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<void> cancelOrder(int orderId) async {
    try {
      await _dio.post('$_baseUrl/orders/$orderId/cancel');
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<Map<String, dynamic>> createPayment(int orderId) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/payments/create',
        data: {
          'order_id': orderId,
        },
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<Map<String, dynamic>> getPaymentStatus(int orderId) async {
    try {
      final response = await _dio.get('$_baseUrl/payments/status/$orderId');
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<Map<String, dynamic>> updateOrderStatus(
      int orderId, String newStatus) async {
    try {
      final response = await _dio.put(
        '$_baseUrl/orders/$orderId/status',
        data: {'status': newStatus},
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<List<String>> getOrderStatuses() async {
    try {
      final response = await _dio.get<List<dynamic>>(
        '$_baseUrl/orders/statuses',
      );
      return response.data!.cast<String>();
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<List<dynamic>> getAllOrders() async {
    try {
      final response = await _dio.get('$_baseUrl/orders/all');
      return response.data as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  String _formatError(DioException e) {
    return e.response != null
        ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
        : 'Сетевая ошибка: ${e.message}';
  }
}

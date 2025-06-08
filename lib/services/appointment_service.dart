import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';
import '../model/appointment.dart';

class AppointmentService {
  final String _baseUrl = dotenv.env['API_BASE_URL']!;
  final Dio _dio;

  AppointmentService(this._dio);

  /// Получить список кабинетов докторов
  Future<List<dynamic>> getDoctorRooms() async {
    try {
      final response = await _dio.get('$_baseUrl/appointments/doctors');
      return response.data as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Получить список кабинетов для анализов
  Future<List<dynamic>> getTestRooms() async {
    try {
      final response = await _dio.get('$_baseUrl/appointments/tests');
      return response.data as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Получить слоты по номеру кабинета
  Future<List<dynamic>> getSlots(int roomNumber, DateTime date) async {
    try {
      final formattedDate =
          date.toIso8601String().split('T')[0]; // "YYYY-MM-DD"
      final response = await _dio.get(
        '$_baseUrl/appointments/slots/$roomNumber',
        queryParameters: {'date': formattedDate},
      );
      return response.data as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Забронировать слот
  Future<Map<String, dynamic>> bookSlot(
      int roomNumber, String appointmentTime) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/appointments/book',
        queryParameters: {
          'room_number': roomNumber,
          'appointment_time': appointmentTime,
        },
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Отменить запись
  Future<Map<String, dynamic>> cancelAppointment(int appointmentId) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/appointments/cancel',
        queryParameters: {
          'appointment_id': appointmentId,
        },
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Получить список моих записей
  Future<List<dynamic>> getMyAppointments() async {
    try {
      final response = await _dio.get('$_baseUrl/appointments/my');
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

  Future<List<Appointment>> getDoctorAppointments() async {
    final response =
        await _dio.get('$_baseUrl/appointments/doctor/appointments');
    final data = response.data as List<dynamic>;
    return data
        .map((e) => Appointment.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<Map<String, dynamic>> setUserCharacteristics({
    required String gender,
    required int height,
    required int weight,
  }) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/user/characteristics',
        data: {
          'gender': gender,
          'height': height,
          'weight': weight,
        },
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }
}

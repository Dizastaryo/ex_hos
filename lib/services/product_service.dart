import 'dart:io';
import 'package:dio/dio.dart';
import '../models/product.dart';

class ProductService {
  static const _baseUrl = 'http://172.20.10.2:8000';
  final Dio _dio = Dio();

  Future<Product> addProduct({
    required ProductCreate product,
    required List<File> images,
    String? token,
  }) async {
    try {
      final formData = FormData.fromMap({
        ...product.toJson(),
        'images': await _prepareImages(images),
      });

      final response = await _dio.post(
        '$_baseUrl/products/',
        data: formData,
        options: Options(
          headers: {'Authorization': 'Bearer $token'},
          contentType: 'multipart/form-data',
        ),
      );

      return Product.fromJson(response.data);
    } on DioException catch (e) {
      throw _handleDioError(e);
    } catch (e) {
      throw 'Ошибка создания продукта: $e';
    }
  }

  Future<List<Product>> getProducts({
    String? category,
    String? search,
    String? token,
  }) async {
    try {
      final response = await _dio.get(
        '$_baseUrl/products/',
        queryParameters: {
          'category': category,
          'search': search,
        },
        options: Options(
          headers: {'Authorization': 'Bearer $token'},
        ),
      );

      return (response.data as List)
          .map((json) => Product.fromJson(json))
          .toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Future<Product> updateProduct({
    required int id,
    required ProductCreate product,
    required List<File> images,
    String? token,
  }) async {
    try {
      final formData = FormData.fromMap({
        ...product.toJson(),
        'images': await _prepareImages(images),
      });

      final response = await _dio.put(
        '$_baseUrl/products/$id',
        data: formData,
        options: Options(
          headers: {'Authorization': 'Bearer $token'},
          contentType: 'multipart/form-data',
        ),
      );

      return Product.fromJson(response.data);
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Future<void> deleteProduct(int id, {String? token}) async {
    try {
      await _dio.delete(
        '$_baseUrl/products/$id',
        options: Options(
          headers: {'Authorization': 'Bearer $token'},
        ),
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Future<List<MultipartFile>> _prepareImages(List<File> images) async {
    return await Future.wait(
      images.map((file) async {
        final path = file.path;
        if (path.isEmpty) throw 'Неверный путь к файлу';

        return await MultipartFile.fromFile(
          path,
          filename: path.split('/').last,
        );
      }),
    );
  }

  String _handleDioError(DioException e) {
    if (e.response != null) {
      return 'Ошибка ${e.response!.statusCode}: ${e.response!.data}';
    }
    return 'Сетевая ошибка: ${e.message}';
  }
}

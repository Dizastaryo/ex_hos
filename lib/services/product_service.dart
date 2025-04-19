import 'dart:io';
import 'package:dio/dio.dart';
import '../models/product.dart';

class ProductService {
  static const _baseUrl = 'http://172.20.10.2:8000';
  final Dio _dio;

  ProductService(this._dio);

  Future<Product> addProduct({
    required ProductCreate product,
    required List<File> images,
  }) async {
    try {
      final formData = await _buildFormData(product, images);

      final response = await _dio.post(
        '$_baseUrl/products/',
        data: formData,
        options: Options(contentType: 'multipart/form-data'),
      );

      return Product.fromJson(response.data);
    } on DioException catch (e) {
      throw _formatError(e);
    } catch (e) {
      throw 'Ошибка при создании продукта: $e';
    }
  }

  Future<List<Product>> getProducts({String? category, String? search}) async {
    try {
      final response = await _dio.get(
        '$_baseUrl/products/',
        queryParameters: {
          if (category != null) 'category': category,
          if (search != null) 'search': search,
        },
      );

      return (response.data as List)
          .map((item) => Product.fromJson(item))
          .toList();
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<Product> updateProduct({
    required int id,
    required ProductCreate product,
    required List<File> images,
  }) async {
    try {
      final formData = await _buildFormData(product, images);

      final response = await _dio.put(
        '$_baseUrl/products/$id',
        data: formData,
        options: Options(contentType: 'multipart/form-data'),
      );

      return Product.fromJson(response.data);
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<void> deleteProduct(int id) async {
    try {
      await _dio.delete('$_baseUrl/products/$id');
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<FormData> _buildFormData(
      ProductCreate product, List<File> images) async {
    final imageFiles = await Future.wait(images.map(
      (file) => MultipartFile.fromFile(
        file.path,
        filename: file.path.split('/').last,
      ),
    ));

    return FormData.fromMap({
      ...product.toJson(),
      'images': imageFiles,
    });
  }

  String _formatError(DioException e) {
    return e.response != null
        ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
        : 'Сетевая ошибка: ${e.message}';
  }
}

import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'dart:io';
import 'package:provider/provider.dart';

import '../services/product_service.dart';
import '../services/category_service.dart';
import '../models/product.dart';
import '../models/category.dart';

class AddProductScreen extends StatefulWidget {
  const AddProductScreen({Key? key}) : super(key: key);

  @override
  _AddProductScreenState createState() => _AddProductScreenState();
}

class _AddProductScreenState extends State<AddProductScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _priceController = TextEditingController();
  int? _selectedCategoryId;
  List<Category> _categories = [];
  List<File> _imageFiles = [];
  final ImagePicker _picker = ImagePicker();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadCategories();
    });
  }

  Future<void> _loadCategories() async {
    try {
      final categoryService =
          Provider.of<CategoryService>(context, listen: false);
      final categories = await categoryService.getCategories();
      setState(() => _categories = categories);
    } catch (e) {
      _showError('Ошибка загрузки категорий: $e');
    }
  }

  Future<void> _pickImages() async {
    try {
      final List<XFile>? pickedFiles = await _picker.pickMultiImage(
        maxWidth: 1024,
        maxHeight: 1024,
        imageQuality: 85,
      );

      if (pickedFiles != null) {
        setState(() {
          _imageFiles.addAll(pickedFiles.map((f) => File(f.path)).toList());
        });
      }
    } catch (e) {
      _showError('Ошибка выбора изображений: $e');
    }
  }

  Future<void> _submitForm() async {
    if (!_formKey.currentState!.validate() || _selectedCategoryId == null)
      return;

    try {
      final product = ProductCreate(
        name: _nameController.text.trim(),
        description: _descriptionController.text.trim(),
        price: double.parse(_priceController.text.trim()),
        categoryId: _selectedCategoryId!,
      );

      final productService =
          Provider.of<ProductService>(context, listen: false);

      await productService.addProduct(
        product: product,
        images: _imageFiles,
      );

      Navigator.pop(context, true);
    } catch (e) {
      _showError('Ошибка создания продукта: $e');
    }
  }

  void _showError(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message)),
    );
  }

  Widget _buildImagePreview() {
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        crossAxisSpacing: 4,
        mainAxisSpacing: 4,
      ),
      itemCount: _imageFiles.length,
      itemBuilder: (ctx, index) => Stack(
        children: [
          Image.file(
            _imageFiles[index],
            fit: BoxFit.cover,
            width: double.infinity,
            height: double.infinity,
          ),
          Positioned(
            right: 0,
            child: IconButton(
              icon: const Icon(Icons.close, color: Colors.red),
              onPressed: () => setState(() => _imageFiles.removeAt(index)),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Добавить продукт')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Form(
          key: _formKey,
          child: ListView(
            children: [
              TextFormField(
                controller: _nameController,
                decoration: const InputDecoration(labelText: 'Название'),
                validator: (v) => v!.isEmpty ? 'Обязательное поле' : null,
              ),
              TextFormField(
                controller: _descriptionController,
                decoration: const InputDecoration(labelText: 'Описание'),
                validator: (v) => v!.isEmpty ? 'Обязательное поле' : null,
              ),
              TextFormField(
                controller: _priceController,
                decoration: const InputDecoration(labelText: 'Цена'),
                keyboardType: TextInputType.number,
                validator: (v) => v!.isEmpty || double.tryParse(v) == null
                    ? 'Некорректная цена'
                    : null,
              ),
              DropdownButtonFormField<int>(
                value: _selectedCategoryId,
                decoration: const InputDecoration(labelText: 'Категория'),
                items: _categories
                    .map((c) => DropdownMenuItem(
                          value: c.id,
                          child: Text(c.name),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _selectedCategoryId = v),
                validator: (v) => v == null ? 'Выберите категорию' : null,
              ),
              const SizedBox(height: 16),
              ElevatedButton.icon(
                icon: const Icon(Icons.photo_library),
                label: const Text('Добавить изображения'),
                onPressed: _pickImages,
              ),
              if (_imageFiles.isNotEmpty) ...[
                const SizedBox(height: 16),
                const Text('Выбранные изображения:'),
                _buildImagePreview(),
              ],
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: _submitForm,
                child: const Text('Создать продукт'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

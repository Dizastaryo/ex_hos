import 'package:flutter/material.dart';
import '../models/box_model.dart';

class BoxScreen extends StatelessWidget {
  final BoxModel box;

  const BoxScreen({super.key, required this.box});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Детали коробки: ${box.id}'),
        backgroundColor: const Color(0xFF6C9942),
      ),
      body: Center(
        child: Text('Вы выбрали коробку ${box.id}, тип: ${box.type}'),
      ),
    );
  }
}

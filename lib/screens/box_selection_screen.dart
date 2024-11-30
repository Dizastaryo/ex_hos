import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/box_provider.dart';
import 'box_screen.dart';

class BoxSelectionScreen extends StatelessWidget {
  const BoxSelectionScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final boxes = Provider.of<BoxProvider>(context).boxes;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Выбор бокса для хранение'),
        backgroundColor: const Color(0xFF6C9942),
      ),
      body: GridView.builder(
        padding: const EdgeInsets.all(16),
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 2,
          crossAxisSpacing: 16,
          mainAxisSpacing: 16,
          childAspectRatio: 1,
        ),
        itemCount: boxes.length,
        itemBuilder: (context, index) {
          final box = boxes[index];
          return GestureDetector(
            onTap: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => BoxScreen(boxModel: box),
                ),
              );
            },
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeInOut,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(16),
                gradient: LinearGradient(
                  colors: box.isAvailable
                      ? [Colors.green[300]!, Colors.green[100]!]
                      : [Colors.red[300]!, Colors.red[100]!],
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                ),
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withOpacity(0.2),
                    blurRadius: 10,
                    offset: const Offset(0, 4),
                  ),
                ],
              ),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(
                    box.id,
                    style: const TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: Colors.white,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '${box.width}x${box.height} см',
                    style: const TextStyle(
                      fontSize: 14,
                      color: Colors.white,
                    ),
                  ),
                  const SizedBox(height: 16),
                  Icon(
                    box.isAvailable ? Icons.check : Icons.close,
                    color: Colors.white,
                    size: 36,
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }
}

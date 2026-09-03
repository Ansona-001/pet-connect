import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/social/application/social_controller.dart';
import 'package:frontend/features/social/data/mock_social_data.dart';

void main() {
  group('SocialController', () {
    test('toggles feed likes and saved state', () {
      final controller = SocialController();
      final post = controller.state.posts.first;

      controller.toggleLike(post.id);
      controller.toggleSave(post.id);

      final updated = controller.state.posts.first;
      expect(updated.isLiked, isTrue);
      expect(updated.likes, post.likes + 1);
      expect(updated.isSaved, isTrue);
    });

    test('creates the seeded reciprocal match and advances the deck', () {
      final controller = SocialController();

      controller.reactToCandidate(interested: true);

      expect(controller.state.lastMatch?.id, 'coco');
      expect(controller.state.candidateIndex, 1);
    });

    test('exhausts and explicitly resets the match deck', () {
      final controller = SocialController();

      for (
        var index = 0;
        index < MockSocialData.matchCandidates.length;
        index++
      ) {
        controller.reactToCandidate(interested: false);
      }

      expect(
        controller.state.candidateIndex,
        MockSocialData.matchCandidates.length,
      );
      controller.reactToCandidate(interested: false);
      expect(
        controller.state.candidateIndex,
        MockSocialData.matchCandidates.length,
      );

      controller.resetMatchDeck();
      expect(controller.state.candidateIndex, 0);
    });

    test('records a premium super like', () {
      final controller = SocialController();

      controller.reactToCandidate(interested: true, superLike: true);

      expect(controller.state.superLikedPetIds, contains('coco'));
    });

    test('persists setup identity and active pet selection', () {
      final controller = SocialController();

      controller.completeSetup(
        ownerName: '  Noor  ',
        city: '  Abu Dhabi ',
        petName: ' Mochi ',
        petType: 'Cat',
        interests: const ['Adoption', 'Pet-friendly cafés'],
      );
      controller.selectActivePet(1);

      expect(controller.state.userProfile.ownerName, 'Noor');
      expect(controller.state.userProfile.city, 'Abu Dhabi');
      expect(controller.state.userProfile.petName, 'Mochi');
      expect(controller.state.userProfile.petType, 'Cat');
      expect(controller.state.activePetIndex, 1);
    });

    test('marks a conversation as read', () {
      final controller = SocialController();

      controller.markChatRead('chat-coco');

      expect(controller.state.readChatIds, contains('chat-coco'));
    });

    test('appends non-empty local chat messages', () {
      final controller = SocialController();
      final initialCount = controller.state.messagesFor('chat-milo').length;

      controller.sendMessage('chat-milo', '  See you Saturday!  ');
      controller.sendMessage('chat-milo', 'Another quick message');
      controller.sendMessage('chat-milo', '   ');

      final messages = controller.state.messagesFor('chat-milo');
      expect(messages, hasLength(initialCount + 2));
      expect(messages[messages.length - 2].text, 'See you Saturday!');
      expect(messages.last.isMine, isTrue);
      expect(messages[messages.length - 2].id, isNot(messages.last.id));
      expect(
        controller.state.messagesFor('chat-coco'),
        MockSocialData.messages,
      );
    });
  });
}

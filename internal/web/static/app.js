// Digitalisation PM — client behaviour.
// Kept deliberately small: htmx boosts navigation; this file adds
// kanban drag & drop, flash auto-dismiss, and confirm prompts.

(function () {
  'use strict';

  // Auto-dismiss flash messages.
  function armFlash(root) {
    (root || document).querySelectorAll('.flash').forEach(function (el) {
      setTimeout(function () {
        el.style.transition = 'opacity .4s';
        el.style.opacity = '0';
        setTimeout(function () { el.remove(); }, 450);
      }, 5000);
    });
  }

  // Kanban drag & drop: cards carry data-task-id, columns data-status.
  function armKanban(root) {
    var scope = root || document;
    scope.querySelectorAll('.kcard[draggable]').forEach(function (card) {
      card.addEventListener('dragstart', function (e) {
        card.classList.add('dragging');
        e.dataTransfer.setData('text/plain', card.dataset.taskId);
        e.dataTransfer.effectAllowed = 'move';
      });
      card.addEventListener('dragend', function () {
        card.classList.remove('dragging');
      });
    });
    scope.querySelectorAll('.kanban-col').forEach(function (col) {
      col.addEventListener('dragover', function (e) {
        e.preventDefault();
        col.classList.add('drag-over');
      });
      col.addEventListener('dragleave', function () {
        col.classList.remove('drag-over');
      });
      col.addEventListener('drop', function (e) {
        e.preventDefault();
        col.classList.remove('drag-over');
        var taskId = e.dataTransfer.getData('text/plain');
        if (!taskId) return;
        var body = new URLSearchParams();
        body.set('status', col.dataset.status);
        fetch('/tasks/' + taskId + '/status', {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: body.toString()
        }).then(function () {
          // Re-render the board region via htmx for a smooth swap.
          if (window.htmx) {
            htmx.ajax('GET', window.location.pathname + window.location.search, {
              target: '#page', select: '#page', swap: 'outerHTML'
            });
          } else {
            window.location.reload();
          }
        });
      });
    });
  }

  function armAll(root) {
    armFlash(root);
    armKanban(root);
  }

  document.addEventListener('DOMContentLoaded', function () { armAll(document); });
  document.body.addEventListener('htmx:afterSwap', function (e) { armAll(e.target); });
})();
